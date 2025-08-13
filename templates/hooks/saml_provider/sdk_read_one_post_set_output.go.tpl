	// Get the ARN from the resource metadata
	if ko.Status.ACKResourceMetadata != nil && ko.Status.ACKResourceMetadata.ARN != nil {
		// Retrieve tags for the SAMLProvider and set them in the resource
		arn := string(*ko.Status.ACKResourceMetadata.ARN)
		ko.Spec.Tags = rm.getTags(ctx, arn)
	}