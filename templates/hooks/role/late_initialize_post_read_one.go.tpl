	// Hold ACK.ResourceSynced=False on a freshly created Role while the
	// `iam.services.k8s.aws/ready-after-create` window has not elapsed. This
	// runs only once late initialization is complete, so it never masks the
	// generated late-initialization requeue below.
	if lateInitializedRes := rm.lateInitializeFromReadOneOutput(observed, latestCopy); !rm.incompleteLateInitialization(lateInitializedRes) {
		if waitingRes, err := rm.waitForPropagation(ctx, lateInitializedRes); err != nil {
			lateInitConditionMessage = "Late initialization successful"
			lateInitConditionReason = "Late initialization successful"
			ackcondition.SetLateInitialized(waitingRes, corev1.ConditionTrue, &lateInitConditionMessage, &lateInitConditionReason)
			return waitingRes, err
		}
	}
