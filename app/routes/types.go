package routes

type API_AccountLogin_PostData struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
