package port

// TokenIssuer abstrai a emissão e validação de tokens de autenticação (ex: JWT).
type TokenIssuer interface {
	Issue(userID string) (string, error)
	Parse(token string) (userID string, err error)
}
