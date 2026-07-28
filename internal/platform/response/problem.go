package response

type FieldErrors map[string][]string

// Problem is the stable public error representation. The internal cause is
// deliberately unexported and therefore cannot be serialized.
type Problem struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Fields    FieldErrors `json:"fields"`
	RequestID string      `json:"request_id"`

	cause error
}

func NewProblem(
	code string,
	message string,
	fields FieldErrors,
	cause error,
) Problem {
	if fields == nil {
		fields = FieldErrors{}
	}

	return Problem{
		Code:    code,
		Message: message,
		Fields:  fields,
		cause:   cause,
	}
}

func (problem Problem) Cause() error {
	return problem.cause
}
