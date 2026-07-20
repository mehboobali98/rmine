package redmine

// User is a Redmine user.
type User struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
	Mail      string `json:"mail"`
}

type currentUserResponse struct {
	User User `json:"user"`
}

// Whoami returns the user associated with the client's API key.
func (c *Client) Whoami() (*User, error) {
	var resp currentUserResponse
	if err := c.get("/users/current.json", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}
