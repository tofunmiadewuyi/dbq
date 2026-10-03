package secrets

const (
	DatabasePasswordEnv = "DBQ_DB_PASSWORD"
	StorageAccessKeyEnv = "DBQ_STORAGE_ACCESS_KEY"
	StorageSecretKeyEnv = "DBQ_STORAGE_SECRET_KEY"
)

// Environment resolves credentials supplied for one externally managed run.
// It does not persist secrets or initialize a local store. The caller owns
// validation of which credentials the requested operation requires.
type Environment struct {
	getenv func(string) string
}

var _ Provider = (*Environment)(nil)

// NewEnvironment accepts the environment reader used by the agent transport.
// These credentials belong to the current invocation, rather than a job ID.
func NewEnvironment(getenv func(string) string) *Environment {
	return &Environment{getenv: getenv}
}

func (p *Environment) Load(_ string) (JobSecrets, error) {
	return JobSecrets{
		DatabasePassword: p.getenv(DatabasePasswordEnv),
		StorageAccessKey: p.getenv(StorageAccessKeyEnv),
		StorageSecretKey: p.getenv(StorageSecretKeyEnv),
	}, nil
}
