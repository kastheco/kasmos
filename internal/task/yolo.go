package task

type YoloSource struct{}

func (s *YoloSource) Type() string          { return "yolo" }
func (s *YoloSource) Path() string          { return "" }
func (s *YoloSource) Load() ([]Task, error) { return nil, nil }
func (s *YoloSource) Tasks() []Task         { return nil }
