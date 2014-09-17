package infrastructure

type awsMetadataServiceProvider struct {
	resolver DNSResolver
}

func NewAwsMetadataServiceProvider(resolver DNSResolver) awsMetadataServiceProvider {
	return awsMetadataServiceProvider{
		resolver: resolver,
	}
}

func (msp awsMetadataServiceProvider) Get() MetadataService {
	return NewHTTPMetadataService(
		"http://169.254.169.254",
		msp.resolver,
	)
}
