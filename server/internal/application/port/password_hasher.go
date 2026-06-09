package port

// PasswordHasher abstrai o algoritmo de hashing de senha (ex: Argon2id + pepper).
// A camada de aplicação nunca conhece o algoritmo concreto.
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) error
}
