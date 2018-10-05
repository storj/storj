package config

type Credentials struct {
	Endpoint  string
	AccessKey string
	SecretKey string
}

type Config struct {
	DefaultOptions   *DefaultOptions
	Server1          *Credentials
	Server2          *Credentials
	ListOptions      *ListOptions
	PutOptions       *PutOptions
	GetObjectOptions *GetObjectOptions
	CopyOptions      *CopyOptions
	DeleteOptions    *DeleteOptions
}

type DefaultOptions struct {
	DefaultSource    string
	ThrowImmediately bool
}

type ListOptions struct {
	DefaultOptions *DefaultOptions
	Merge          bool
}

type PutOptions struct {
	DefaultOptions         *DefaultOptions
	CreateBucketIfNotExist bool
}

type GetObjectOptions struct {
	DefaultOptions *DefaultOptions
}

type CopyOptions struct {
	DefaultOptions *DefaultOptions
}

type DeleteOptions struct {
	DefaultOptions *DefaultOptions
}

// Creates new instance of Config
func NewConfig() *Config {

	return &Config{}
}

func (c *Config) WithDefaultOptions(defaultOptions *DefaultOptions) *Config {
	c.DefaultOptions = defaultOptions

	return c
}

func (c *Config) WithServer1Credentials(credentials *Credentials) *Config {
	c.Server1 = credentials

	return c
}

func (c *Config) WithServer2Credentials(credentials *Credentials) *Config {
	c.Server2 = credentials

	return c
}

func (c *Config) WithListOptions(options *DefaultOptions, merge bool) *Config {
	c.ListOptions = &ListOptions{
		DefaultOptions: options,
		Merge:          merge,
	}

	return c
}

func (c *Config) WithPutOptions(options *DefaultOptions, createBucketIfNotExist bool) *Config {
	c.PutOptions = &PutOptions{
		DefaultOptions:         options,
		CreateBucketIfNotExist: createBucketIfNotExist,
	}

	return c
}

func (c *Config) WithGetObjectOptions(options *DefaultOptions) *Config {
	c.GetObjectOptions = &GetObjectOptions{
		DefaultOptions: options,
	}

	return c
}

func (c *Config) WithDeleteOptions(options *DefaultOptions) *Config {
	c.DeleteOptions = &DeleteOptions{
		DefaultOptions: options,
	}

	return c
}

func (c *Config) WithCopyOptions(options *DefaultOptions) *Config {
	c.CopyOptions = &CopyOptions{
		DefaultOptions: options,
	}

	return c
}

func NewCredentials(endpoint string, accessKey string, secretKey string) *Credentials {

	return &Credentials{
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
}

func NewDefaultOptions(defaultSource string, throwImmediately bool) *DefaultOptions {

	return &DefaultOptions{
		DefaultSource:    defaultSource,
		ThrowImmediately: throwImmediately}
}

func (do *DefaultOptions) GetDefaultSource(cdo *DefaultOptions) string {
	if cdo == nil || cdo.DefaultSource == "" {

		return do.DefaultSource
	}

	return cdo.DefaultSource
}

func (do *DefaultOptions) IsThrowImmediateError(cdo *DefaultOptions) bool {
	if cdo == nil {

		return do.ThrowImmediately
	}

	return cdo.ThrowImmediately
}

func (c *Config) getPrimeCredentials(server string) *Credentials {
	switch server {
	case "server1":

		return c.Server1
	case "server2":

		return c.Server2
	default:

		return nil

	}
}

func (c *Config) getAlterCredentials(server string) *Credentials {
	switch server {
	case "server1":

		return c.Server2
	case "server2":

		return c.Server1
	default:

		return nil
	}
}

func (c *Config) GetMergedPrimeCredentials(options *DefaultOptions) *Credentials {
	if options != nil && "" != options.DefaultSource {

		return c.getPrimeCredentials(options.DefaultSource)
	}

	return c.getPrimeCredentials(c.DefaultOptions.DefaultSource)
}

func (c *Config) GetMergedAlterCredentials(options *DefaultOptions) *Credentials {
	if options != nil && "" != options.DefaultSource {

		return c.getAlterCredentials(options.DefaultSource)
	}

	return c.getAlterCredentials(c.DefaultOptions.DefaultSource)
}

func (c *Credentials) IsEmpty() bool {
	return "" == c.Endpoint || "" == c.AccessKey || "" == c.SecretKey
}
