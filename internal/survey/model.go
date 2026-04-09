package survey

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("survey not found")

type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusClosed   Status = "closed"
	StatusArchived Status = "archived"
)

type QuestionType string

const (
	TypeStars        QuestionType = "stars"
	TypeSchulnote    QuestionType = "schulnote"
	TypeText         QuestionType = "text"
	TypeSingleChoice QuestionType = "single_choice"
	TypeMultiChoice  QuestionType = "multi_choice"
)

type Question struct {
	ID       string       `json:"id"`
	Type     QuestionType `json:"type"`
	Text     string       `json:"text"`
	Required bool         `json:"required"`
	Standard bool         `json:"standard"`
	Options  []string     `json:"options,omitempty"`
}

type Survey struct {
	ID              int
	EveningID       int
	Status          Status
	Questions       []Question
	CloseAfterHours *int
	ActivatedAt     *time.Time
	ClosesAt        *time.Time
	ClosedAt        *time.Time
	CreatedAt       time.Time
}

type Response struct {
	ID          int
	SurveyID    int
	Answers     map[string]any
	SubmittedAt time.Time
}
