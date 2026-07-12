package git

import (
	"fmt"
	"strings"
)

// ValidateVerification confirms that both the task branch head and its diff
// base still match the commits reviewed by the verifier.
func ValidateVerification(repoPath, branch, verifiedHead, verifiedBase string) (head, base, reason string, err error) {
	head, err = BranchHeadSHA(repoPath, branch)
	if err != nil {
		return "", "", "", err
	}
	base, err = DefaultBranchHeadSHA(repoPath)
	if err != nil {
		return "", "", "", err
	}
	if verifiedHead == "" || !strings.EqualFold(verifiedHead, head) {
		return head, base, fmt.Sprintf("head_changed_after_verification: verified %s, head is now %s", ShortSHA(verifiedHead), ShortSHA(head)), nil
	}
	// Empty base is a legacy pre-binding record. New approvals always persist a
	// base SHA; once present it is enforced as part of the verification proof.
	if verifiedBase != "" && !strings.EqualFold(verifiedBase, base) {
		return head, base, fmt.Sprintf("base_changed_after_verification: verified %s, base is now %s", ShortSHA(verifiedBase), ShortSHA(base)), nil
	}
	return head, base, "", nil
}
