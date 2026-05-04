package hook

import (
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

// stageConversation links orphan messages to the step being created
// Messages are captured by UserPromptSubmit, Stop, and PostToolBatch hooks
func stageConversation(s *store.Store, idx *index.DB, p Payload, stepHash store.Hash) error {
	// Link all orphan messages (step_id IS NULL) to this step
	return idx.LinkMessagesToStep(p.SessionID, stepHash)
}
