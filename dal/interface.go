package dal

type DAL interface {
	// List functions created by a user
	ListFunctionsOfUser(username string, userId int64) ([]*Function, error)

	// Insert user into DB if not existed.
	//
	// Returns: (int64) insert row id,
	//          (int64) # of rows influenced,
	//          (error) if there is one
	PutUserIfNotExisted(groupName, userName string) (int64, int64, error)

	// Put the function into the DB
	// If the function does not exist, insert one,
	// otherwise, update it.
	//
	// Returns: (int64) insert row id,
	//          (int64) # of rows influenced,
	//          (error) if there is one
	PutFunction(userName, funcName, funcContent string, userId int64) (int64, int64, error)

	// Get the content of a function
	//
	// Returns: (string) function content
	//			(error) if there is one
	GetFunction(userName, funcName string) (string, error)

	// Delete the function from the DB
	//
	// Returns: (error) if there is one
	DeleteFunction(userName, funcName string) error

	// Clear content from all tables
	// Returns: (error) if there is one
	ClearDatabase() error
}
