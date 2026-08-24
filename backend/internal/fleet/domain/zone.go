package domain

type Zone struct {
	ID          string
	Name        string
	Coordinates [][]float64
}

func (z Zone) Validate() error {
	if err := validateUUID(z.ID); err != nil {
		return err
	}
	if err := validateZoneName(z.Name); err != nil {
		return err
	}
	roundCoords(z.Coordinates)
	if err := validateCoordinatesCountClosure(z.Coordinates); err != nil {
		return err
	}
	if err := validatePolygonRange(z.Coordinates); err != nil {
		return err
	}
	if err := validateSelfIntersection(z.Coordinates); err != nil {
		return err
	}
	if err := validateArea(z.Coordinates); err != nil {
		return err
	}
	return nil
}
