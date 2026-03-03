package uploadapi

import (
	"errors"
	"strconv"
	"strings"
)

type jobIDURI struct {
	ID string `uri:"id" binding:"required"`
}

type presignPartsQuery struct {
	Parts string `form:"parts" binding:"required"`
}

func parsePartNumbers(raw string) ([]int32, error) {
	tokens := strings.Split(raw, ",")
	out := make([]int32, 0, len(tokens))
	for _, t := range tokens {
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 32)
		if err != nil || n < 1 {
			return nil, errors.New("each part number must be a positive integer")
		}
		out = append(out, int32(n))
	}
	return out, nil
}
