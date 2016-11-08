package dal

import "time"

type Group struct {
	ID      int64
	Name    string
	Created time.Time
	Users   []User
}

type User struct {
	ID      int64
	Name    string
	Created time.Time
}

type Function struct {
	ID      int64
	UserID  int64
	Name    string
	Content string
	Updated time.Time
}

type FunctionExecution struct {
	ID         int64
	FunctionID int64
	Log        string
	Timestamp  time.Time
}
