package app

type Middleware_Session_JWT struct {
	SessionID string `json:"sessionId"`
	AccountID string `json:"accountId"`
	UserID    string `json:"userId"`
}

type API_AccountLogin_PostData struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type API_AccountSignup_PostData struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	AccountName string `json:"accountName"`
}
type API_AccountCreateAgent_PostData struct {
	AgentName string `json:"agentName"`
	Port      int    `json:"port"`
	Memory    int64  `json:"memory"`
}

type API_AccountUserTwoFACode_PostData struct {
	Token string `json:"token"`
}

type API_AccountAgentTask_PostData struct {
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}
