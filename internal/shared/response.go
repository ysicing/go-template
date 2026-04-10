package shared

type Response struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func OK(data any) Response {
	return Response{
		Code:    "OK",
		Message: "success",
		Data:    data,
	}
}

func Err(code string, message string) Response {
	return Response{
		Code:    code,
		Message: message,
	}
}

