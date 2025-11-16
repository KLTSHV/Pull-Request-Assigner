package random

import (
	"math/rand"
	"time"

	"PullRequestAssigner/internal/domain"
)

// Selector абстракция выбора случайных ревьюверов.
type Selector interface {
	// PickReviewers выбирает до max ревьюверов из кандидатов
	PickReviewers(candidates []domain.UserID, max int) []domain.UserID
}

// DefaultSelector реализация Selector
type DefaultSelector struct {
	rnd *rand.Rand
}

// NewDefaultSelector создаёт селектор с нормальным источником случайности.
func NewDefaultSelector() *DefaultSelector {
	src := rand.NewSource(time.Now().UnixNano())
	return &DefaultSelector{
		rnd: rand.New(src),
	}
}

// PickReviewers выбор случайных ревьюверов без повторов (Фишер-Йетс по индексам
func (s *DefaultSelector) PickReviewers(candidates []domain.UserID, max int) []domain.UserID {
	return pickRandomWithoutReplacement(s.rnd, candidates, max)
}

// pickRandomWithoutReplacement вспомогательная функция, чтобы можно было переиспользовать логику
func pickRandomWithoutReplacement(r *rand.Rand, items []domain.UserID, max int) []domain.UserID {
	n := len(items)
	if n == 0 || max <= 0 {
		return nil
	}
	if n <= max {
		// возвращаем копию, чтобы не мутировать исходный слайс
		res := make([]domain.UserID, n)
		copy(res, items)
		return res
	}

	// Перемешиваем индексы и берём первые max.
	indexes := make([]int, n)
	for i := range items {
		indexes[i] = i
	}

	for i := n - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		indexes[i], indexes[j] = indexes[j], indexes[i]
	}

	res := make([]domain.UserID, 0, max)
	for i := 0; i < max; i++ {
		res = append(res, items[indexes[i]])
	}
	return res
}
