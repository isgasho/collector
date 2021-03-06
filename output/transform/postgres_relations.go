package transform

import (
	"github.com/golang/protobuf/ptypes"
	"github.com/guregu/null"
	snapshot "github.com/pganalyze/collector/output/pganalyze_collector"
	"github.com/pganalyze/collector/state"
)

func transformPostgresRelations(s snapshot.FullSnapshot, newState state.PersistedState, diffState state.DiffState, roleOidToIdx OidToIdx, databaseOidToIdx OidToIdx) snapshot.FullSnapshot {
	for _, relation := range newState.Relations {
		ref := snapshot.RelationReference{
			DatabaseIdx:  databaseOidToIdx[relation.DatabaseOid],
			SchemaName:   relation.SchemaName,
			RelationName: relation.RelationName,
		}
		idx := int32(len(s.RelationReferences))
		s.RelationReferences = append(s.RelationReferences, &ref)

		// Information
		info := snapshot.RelationInformation{
			RelationIdx:            idx,
			RelationType:           relation.RelationType,
			PersistenceType:        relation.PersistenceType,
			Fillfactor:             relation.Fillfactor(),
			HasOids:                relation.HasOids,
			HasInheritanceChildren: relation.HasInheritanceChildren,
			HasToast:               relation.HasToast,
			FrozenXid:              uint32(relation.FrozenXID),
			MinimumMultixactXid:    uint32(relation.MinimumMultixactXID),
			ExclusivelyLocked:      relation.ExclusivelyLocked,
			Options:                relation.Options,
		}

		if relation.ViewDefinition != "" {
			info.ViewDefinition = &snapshot.NullString{Valid: true, Value: relation.ViewDefinition}
		}
		for _, column := range relation.Columns {
			sColumn := snapshot.RelationInformation_Column{
				Name:     column.Name,
				DataType: column.DataType,
				NotNull:  column.NotNull,
				Position: column.Position,
			}
			if column.DefaultValue.Valid {
				sColumn.DefaultValue = &snapshot.NullString{Valid: true, Value: column.DefaultValue.String}
			}
			info.Columns = append(info.Columns, &sColumn)
		}
		for _, constraint := range relation.Constraints {
			sConstraint := snapshot.RelationInformation_Constraint{
				Name:              constraint.Name,
				Type:              constraint.Type,
				ConstraintDef:     constraint.ConstraintDef,
				ForeignUpdateType: constraint.ForeignUpdateType,
				ForeignDeleteType: constraint.ForeignDeleteType,
				ForeignMatchType:  constraint.ForeignMatchType,
			}
			if constraint.ForeignOid != 0 {
				sConstraint.ForeignRelationIdx = -1 // FIXME, need to look this up
			}
			for _, column := range constraint.Columns {
				sConstraint.Columns = append(sConstraint.Columns, int32(column))
			}
			for _, column := range constraint.ForeignColumns {
				sConstraint.ForeignColumns = append(sConstraint.ForeignColumns, int32(column))
			}
			info.Constraints = append(info.Constraints, &sConstraint)
		}
		s.RelationInformations = append(s.RelationInformations, &info)

		// Statistic
		stats, exists := diffState.RelationStats[relation.Oid]
		if exists {
			statistic := snapshot.RelationStatistic{
				RelationIdx:    idx,
				SizeBytes:      stats.SizeBytes,
				ToastSizeBytes: stats.ToastSizeBytes,
				SeqScan:        stats.SeqScan,
				SeqTupRead:     stats.SeqTupRead,
				IdxScan:        stats.IdxScan,
				IdxTupFetch:    stats.IdxTupFetch,
				NTupIns:        stats.NTupIns,
				NTupUpd:        stats.NTupUpd,
				NTupDel:        stats.NTupDel,
				NTupHotUpd:     stats.NTupHotUpd,
				NLiveTup:       stats.NLiveTup,
				NDeadTup:       stats.NDeadTup,
				HeapBlksRead:   stats.HeapBlksRead,
				HeapBlksHit:    stats.HeapBlksHit,
				IdxBlksRead:    stats.IdxBlksRead,
				IdxBlksHit:     stats.IdxBlksHit,
				ToastBlksRead:  stats.ToastBlksRead,
				ToastBlksHit:   stats.ToastBlksHit,
				TidxBlksRead:   stats.TidxBlksRead,
				TidxBlksHit:    stats.TidxBlksHit,
			}
			if stats.NModSinceAnalyze.Valid {
				statistic.NModSinceAnalyze = stats.NModSinceAnalyze.Int64
			}
			s.RelationStatistics = append(s.RelationStatistics, &statistic)

			// Events
			s.RelationEvents = addRelationEvents(idx, s.RelationEvents, stats.AnalyzeCount, stats.LastAnalyze, snapshot.RelationEvent_MANUAL_ANALYZE)
			s.RelationEvents = addRelationEvents(idx, s.RelationEvents, stats.AutoanalyzeCount, stats.LastAutoanalyze, snapshot.RelationEvent_AUTO_ANALYZE)
			s.RelationEvents = addRelationEvents(idx, s.RelationEvents, stats.VacuumCount, stats.LastVacuum, snapshot.RelationEvent_MANUAL_VACUUM)
			s.RelationEvents = addRelationEvents(idx, s.RelationEvents, stats.AutovacuumCount, stats.LastAutovacuum, snapshot.RelationEvent_AUTO_VACUUM)
		}

		// Indices
		for _, index := range relation.Indices {
			ref := snapshot.IndexReference{
				DatabaseIdx: databaseOidToIdx[relation.DatabaseOid],
				SchemaName:  relation.SchemaName,
				IndexName:   index.Name,
			}
			indexIdx := int32(len(s.IndexReferences))
			s.IndexReferences = append(s.IndexReferences, &ref)

			// Information
			indexInfo := snapshot.IndexInformation{
				IndexIdx:    indexIdx,
				RelationIdx: idx,
				IndexType:   index.IndexType,
				IndexDef:    index.IndexDef,
				IsPrimary:   index.IsPrimary,
				IsUnique:    index.IsUnique,
				IsValid:     index.IsValid,
				Fillfactor:  index.Fillfactor(),
			}
			if index.ConstraintDef.Valid {
				indexInfo.ConstraintDef = &snapshot.NullString{Valid: true, Value: index.ConstraintDef.String}
			}
			for _, column := range index.Columns {
				indexInfo.Columns = append(indexInfo.Columns, int32(column))
			}
			s.IndexInformations = append(s.IndexInformations, &indexInfo)

			// Statistic
			indexStats, exists := diffState.IndexStats[index.IndexOid]
			if exists {
				statistic := snapshot.IndexStatistic{
					IndexIdx:    indexIdx,
					SizeBytes:   indexStats.SizeBytes,
					IdxScan:     indexStats.IdxScan,
					IdxTupRead:  indexStats.IdxTupRead,
					IdxTupFetch: indexStats.IdxTupFetch,
					IdxBlksRead: indexStats.IdxBlksRead,
					IdxBlksHit:  indexStats.IdxBlksHit,
				}
				s.IndexStatistics = append(s.IndexStatistics, &statistic)
			}
		}
	}

	return s
}

func addRelationEvents(relationIdx int32, events []*snapshot.RelationEvent, count int64, lastTime null.Time, eventType snapshot.RelationEvent_EventType) []*snapshot.RelationEvent {
	if count == 0 {
		return events
	}

	ts, _ := ptypes.TimestampProto(lastTime.Time)

	for i := int64(0); i < count; i++ {
		event := snapshot.RelationEvent{
			RelationIdx:           relationIdx,
			Type:                  eventType,
			OccurredAt:            ts,
			ApproximateOccurredAt: i != 0,
		}
		events = append(events, &event)
	}

	return events
}
