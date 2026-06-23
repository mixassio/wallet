package domain

import "errors"

// Доменные ошибки, не зависящие от транспортного слоя.
var (
	// запрашиваемая сущность не найдена
	ErrNotFound = errors.New("not found")
	// некорректные входные данные
	ErrValidation = errors.New("validation error")
)
