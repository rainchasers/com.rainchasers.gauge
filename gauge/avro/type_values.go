// Code generated by gopkg.in/actgardner/gogen-avro.v5. DO NOT EDIT.
/*
 * SOURCE:
 *     gauge.avsc
 */

package avro

type TypeValues int32

const (
	Level       TypeValues = 0
	Flow        TypeValues = 1
	Temperature TypeValues = 2
	Rainfall    TypeValues = 3
)

func (e TypeValues) String() string {
	switch e {
	case Level:
		return "level"
	case Flow:
		return "flow"
	case Temperature:
		return "temperature"
	case Rainfall:
		return "rainfall"

	}
	return "Unknown"
}