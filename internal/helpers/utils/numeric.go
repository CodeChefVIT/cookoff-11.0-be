package utils

import (
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

func NumericToFloat64(number pgtype.Numeric) (float64, error) {
	value, err := number.Float64Value()
	if err != nil {
		return 0, err
	}

	if !value.Valid {
		return 0, errors.New("invalid numeric value")
	}

	return value.Float64, nil
}

func Float64ToNumeric(value float64) (pgtype.Numeric, error) {
	var number pgtype.Numeric
	err := number.Scan(strconv.FormatFloat(value, 'f', -1, 64))
	if err != nil {
		return pgtype.Numeric{}, err
	}

	return number, nil
}
