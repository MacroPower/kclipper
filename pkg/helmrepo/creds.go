package helmrepo

type Creds struct {
	Username           string
	Password           string
	CAPath             string
	CertData           []byte
	KeyData            []byte
	InsecureSkipVerify bool
	PassCredentials    bool
}
