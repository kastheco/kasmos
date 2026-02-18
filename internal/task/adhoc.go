package task

type AdHocSource struct{}

func (s *AdHocSource) Type() string          { return "ad-hoc" }
func (s *AdHocSource) Path() string          { return "" }
func (s *AdHocSource) Load() ([]Task, error) { return nil, nil }
func (s *AdHocSource) Tasks() []Task         { return nil }
