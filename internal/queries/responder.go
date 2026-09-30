package queries

import "github.com/shurco/mycart/internal/database"

// ResponderQueries holds queries for the responder system (messages, workflows, contacts)
type ResponderQueries struct {
	DB *database.Conn
}
