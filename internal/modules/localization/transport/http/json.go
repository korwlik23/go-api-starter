package http

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/gin-gonic/gin"
)

// decodeStrictJSON keeps mutation contracts closed: unknown fields and
// multiple JSON values are rejected before a use case is invoked.
func decodeStrictJSON(ctx *gin.Context, destination any) error {
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
