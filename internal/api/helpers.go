package api

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseRange парсит параметр range вида "[start,end]" в start и end
func ParseRange(rangeStr string) (start, end int32, err error) {
	// Убираем квадратные скобки
	trimmed := strings.Trim(rangeStr, "[]")

	// Разделяем по запятой
	parts := strings.Split(trimmed, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	// Парсим числа
	start64, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start value: %w", err)
	}

	end64, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end value: %w", err)
	}

	return int32(start64), int32(end64), nil
}
