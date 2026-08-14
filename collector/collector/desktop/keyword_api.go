package main

import "time"

type KeywordHitDTO struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	DeviceID  string    `json:"deviceId"`
	RuleID    string    `json:"ruleId"`
	RuleName  string    `json:"ruleName"`
	MatchedAt time.Time `json:"matchedAt"`
	Sequence  int64     `json:"sequence"`
	LineText  string    `json:"lineText"`
}

func (s *Service) GetKeywordHits(sessionID, ruleID string) ([]KeywordHitDTO, error) {
	hits, err := s.store.ListKeywordHits(s.ctx, sessionID, ruleID)
	if err != nil {
		return nil, err
	}
	result := make([]KeywordHitDTO, 0, len(hits))
	for _, hit := range hits {
		result = append(result, KeywordHitDTO{ID: hit.ID, SessionID: hit.SessionID, DeviceID: hit.DeviceSN, RuleID: hit.RuleID, RuleName: hit.RuleName, MatchedAt: hit.MatchedAt, Sequence: hit.Sequence, LineText: hit.LineText})
	}
	return result, nil
}

func (s *Service) ResetKeywordHits(sessionID string) error {
	return s.store.ResetKeywordHits(s.ctx, sessionID)
}
