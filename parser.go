package factom_identity

import (
	"fmt"

	"github.com/FactomProject/factomd/common/adminBlock"
	"github.com/FactomProject/factomd/common/constants"
	"github.com/FactomProject/factomd/common/identity"
	"github.com/FactomProject/factomd/common/interfaces"
)

type ExtendedIdentity struct {
	IdentityCore identity.Identity `json:"id_core"`
	Extension    IdentityExtension `json:"id_extension"`
}

// IdentityExtension is the unofficial identity fields
type IdentityExtension struct {
}

// Parser can parse identity related entries or admin blocks. It
// can also be extended to allow for additional entry types (such as naming)
type IdentityParser struct {
	*identity.IdentityManager
	Extensions map[[32]byte]IdentityExtension

	// For quicker lookups
	ManagementChains map[[32]byte]interfaces.IHash
}

func NewIdentityParser() *IdentityParser {
	p := new(IdentityParser)

	p.IdentityManager = identity.NewIdentityManager()
	return p
}

func (p *IdentityParser) GetExtendedIdentity(hash interfaces.IHash) *ExtendedIdentity {
	id := p.IdentityManager.GetIdentity(hash)

	if id == nil {
		return nil
	}

	extension := p.Extensions[id.IdentityChainID.Fixed()]
	return &ExtendedIdentity{*id, extension}
}

func (p *IdentityParser) ParseEntryList(list []IdentityEntry) error {
	for _, e := range list {
		p.ParseEntry(e.Entry, e.BlockHeight, e.Timestamp, true)
	}

	// Parse the remaining
	p.ProcessOldEntries()
	return nil
}

// ParseEntry is mostly handled by the IdentityManager, however it can be extended to support additional parsing options (such as naming)
//	Returns
//		changed : IdentityChainID of the identity that has changed
//		err
func (p *IdentityParser) ParseEntry(entry interfaces.IEBEntry, dBlockHeight uint32, dBlockTimestamp interfaces.Timestamp, newEntry bool) (changed interfaces.IHash, err error) {
	var change bool
	change, err = p.ProcessIdentityEntry(entry, dBlockHeight, dBlockTimestamp, newEntry)
	if err != nil {
		return nil, err
	}

	// Change means the entry was consumed to update identity
	if change {
		// Set the chainid of the identity chain id.
		id, ok := p.IdentityManager.Identities[entry.GetChainID().Fixed()]
		if ok {
			changed = entry.GetChainID()
			p.ManagementChains[id.IdentityChainID.Fixed()] = id.ManagementChainID
			return
		}

		v, ok := p.ManagementChains[entry.GetChainID().Fixed()]
		if ok {
			changed = v
			return
		}
		return
	}

	/*******************
		Custom Parsing
	 *******************/

	if entry.GetChainID().String()[:6] != "888888" {
		return nil, fmt.Errorf("Invalic chainID - expected 888888..., got %v", entry.GetChainID().String())
	}
	if entry.GetHash().String() == "172eb5cb84a49280c9ad0baf13bea779a624def8d10adab80c3d007fe8bce9ec" {
		//First entry, can ignore
		return nil, nil
	}

	// Not always the authority chainID, it can be any chain with '8888', so management, authority, or register chain
	chainID := entry.GetChainID()

	extIDs := entry.ExternalIDs()
	if len(extIDs) < 2 {
		//Invalid Identity Chain Entry
		return
	}
	if len(extIDs[0]) == 0 {
		return
	}
	if extIDs[0][0] != 0 {
		//We only support version 0
		return
	}

	// This is the entry's name. The ones detailed in the identity spec are covered above, we can support additional
	// types here
	switch string(extIDs[1]) {
	case "Extended Option Here":

	}

	var _ = chainID

	return
}

// ParseAdminBlockEntry is a bit tricky. It's more coupled with factomd, so we need
// to do a little more overhead to get this to work as intended. First, there is no state,
// so this will NOT work if you have not also processed identity entries.
//
// Second we need to prevent the authority removal from removing the identity too, by catching it first.
func (p *IdentityParser) ParseAdminBlockEntry(ab interfaces.IABEntry) error {
	// Need to catch this one, as the regular function also removes the identity.
	// TODO: Need to correct status
	switch ab.Type() {
	case constants.TYPE_REMOVE_FED_SERVER:
		e := ab.(*adminBlock.RemoveFederatedServer)
		p.RemoveAuthority(e.IdentityChainID)
		return nil
	}

	return p.IdentityManager.ProcessABlockEntry(ab, &FakeState{})
}
