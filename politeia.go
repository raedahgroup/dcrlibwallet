package dcrlibwallet

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"decred.org/dcrwallet/errors"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
)

type Politeia struct {
	mwRef                   *MultiWallet
	host                    string
	mu                      sync.RWMutex
	ctx                     context.Context
	cancelSync              context.CancelFunc
	client                  *politeiaClient
	notificationListenersMu sync.RWMutex
	notificationListeners   map[string]ProposalNotificationListener
}

const (
	proposalsDbName = "proposals.db"
)

const (
	ProposalCategoryAll int32 = iota + 1
	ProposalCategoryPre
	ProposalCategoryActive
	ProposalCategoryApproved
	ProposalCategoryRejected
	ProposalCategoryAbandoned
)

func newPoliteia(mwRef *MultiWallet, host string) (*Politeia, error) {
	p := &Politeia{
		mwRef:                 mwRef,
		host:                  host,
		client:                nil,
		notificationListeners: make(map[string]ProposalNotificationListener),
	}

	return p, nil
}

func (p *Politeia) saveOrOverwiteProposal(proposal *Proposal) error {
	var oldProposal Proposal
	err := p.mwRef.db.One("Token", proposal.Token, &oldProposal)
	if err != nil && err != storm.ErrNotFound {
		return errors.Errorf("error checking if proposal was already indexed: %s", err.Error())
	}

	if oldProposal.Token != "" {
		// delete old record before saving new (if it exists)
		p.mwRef.db.DeleteStruct(oldProposal)
	}

	return p.mwRef.db.Save(proposal)
}

// GetProposalsRaw fetches and returns a proposals from the db
func (p *Politeia) GetProposalsRaw(category int32, offset, limit int32, newestFirst bool) ([]Proposal, error) {
	return p.getProposalsRaw(category, offset, limit, newestFirst, false)
}

func (p *Politeia) getProposalsRaw(category int32, offset, limit int32, newestFirst bool, skipAbandoned bool) ([]Proposal, error) {

	var query storm.Query
	switch category {
	case ProposalCategoryAll:

		if skipAbandoned {
			query = p.mwRef.db.Select(
				q.Not(q.Eq("Category", ProposalCategoryAbandoned)),
			)
		} else {
			query = p.mwRef.db.Select(
				q.True(),
			)
		}
	default:
		query = p.mwRef.db.Select(
			q.Eq("Category", category),
		)
	}

	if offset > 0 {
		query = query.Skip(int(offset))
	}

	if limit > 0 {
		query = query.Limit(int(limit))
	}

	if newestFirst {
		query = query.OrderBy("PublishedAt").Reverse()
	} else {
		query = query.OrderBy("PublishedAt")
	}

	var proposals []Proposal
	err := query.Find(&proposals)
	if err != nil && err != storm.ErrNotFound {
		return nil, fmt.Errorf("error fetching proposals: %s", err.Error())
	}

	return proposals, nil
}

// GetProposals returns the result of GetProposalsRaw as a JSON string
func (p *Politeia) GetProposals(category int32, offset, limit int32, newestFirst bool) (string, error) {

	result, err := p.GetProposalsRaw(category, offset, limit, newestFirst)
	if err != nil {
		return "", err
	}

	if len(result) == 0 {
		return "[]", nil
	}

	response, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("error marshalling result: %s", err.Error())
	}

	return string(response), nil
}

// GetProposalRaw fetches and returns a single proposal specified by it's censorship record token
func (p *Politeia) GetProposalRaw(censorshipToken string) (*Proposal, error) {
	var proposal Proposal
	err := p.mwRef.db.One("Token", censorshipToken, &proposal)
	if err != nil {
		return nil, err
	}

	return &proposal, nil
}

// GetProposal returns the result of GetProposalRaw as a JSON string
func (p *Politeia) GetProposal(censorshipToken string) (string, error) {
	return p.marshalResult(p.GetProposalRaw(censorshipToken))
}

// GetProposalByIDRaw fetches and returns a single proposal specified by it's ID
func (p *Politeia) GetProposalByIDRaw(proposalID int) (*Proposal, error) {
	var proposal Proposal
	err := p.mwRef.db.One("ID", proposalID, &proposal)
	if err != nil {
		return nil, err
	}

	return &proposal, nil
}

// GetProposalByID returns the result of GetProposalByIDRaw as a JSON string
func (p *Politeia) GetProposalByID(proposalID int) (string, error) {
	return p.marshalResult(p.GetProposalByIDRaw(proposalID))
}

// Count returns the number of proposals of a specified category
func (p *Politeia) Count(category int32) (int32, error) {
	var matcher q.Matcher

	if category == ProposalCategoryAll {
		matcher = q.True()
	} else {
		matcher = q.Eq("Category", category)
	}

	count, err := p.mwRef.db.Select(matcher).Count(&Proposal{})
	if err != nil {
		return 0, err
	}

	return int32(count), nil
}

func (p *Politeia) ClearSavedProposals() error {
	err := p.mwRef.db.Drop(&Proposal{})
	if err != nil {
		return translateError(err)
	}

	return p.mwRef.db.Init(&Proposal{})
}

func (p *Politeia) marshalResult(result interface{}, err error) (string, error) {

	if err != nil {
		return "", translateError(err)
	}

	response, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("error marshalling result: %s", err.Error())
	}

	return string(response), nil
}
