package river

// Calibration is a referenced gauge related to a section
type Calibration struct {
	URL         string            `firestore:"data_url"`
	Description string            `firestore:"desc"`
	Minimum     map[Level]float32 `firestore:"-"`
}

// LevelAt provides the level state at a certain reading
func (c Calibration) LevelAt(value float32) Level {
	if len(c.Minimum) == 0 {
		return Unknown
	}

	state := Empty
	for lvl, min := range c.Minimum {
		if value >= min && lvl > state {
			state = lvl
		}
	}

	return state
}