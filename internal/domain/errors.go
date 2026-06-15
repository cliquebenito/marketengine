package domain

import "errors"

var ErrNotFound = errors.New("domain: not found")

var ErrInsufficientCoverage = errors.New("domain: insufficient coverage")

var ErrInvariantViolation = errors.New("domain: invariant violation")
