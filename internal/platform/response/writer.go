package response

import "github.com/gin-gonic/gin"

type envelope struct {
	Error Problem `json:"error"`
}

func WriteProblem(
	ctx *gin.Context,
	status int,
	requestID string,
	problem Problem,
) {
	if problem.Fields == nil {
		problem.Fields = FieldErrors{}
	}
	problem.RequestID = requestID
	ctx.AbortWithStatusJSON(status, envelope{Error: problem})
}
