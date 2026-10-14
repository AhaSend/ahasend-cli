package mocks

import (
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
)

// Fixtures shared by the printer and command tests. Their values are fixed,
// so a test can compare output exactly.

const (
	// FixtureSubAccountID is the ID of NewSubAccount and the account of
	// NewAPIKey.
	FixtureSubAccountID = "11111111-1111-1111-1111-111111111111"
	// FixtureParentAccountID is the parent account of NewSubAccount.
	FixtureParentAccountID = "22222222-2222-2222-2222-222222222222"
	// FixtureAPIKeyID is the ID of NewAPIKey.
	FixtureAPIKeyID = "44444444-4444-4444-4444-444444444444"
	// FixtureDomainID is the ID of NewDomain, which belongs to the account
	// FixtureSubAccountID.
	FixtureDomainID = "55555555-5555-4555-8555-555555555555"
)

// NewSubAccount returns a sub-account with every field set.
func NewSubAccount() *responses.SubAccount {
	lastActivity := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	return &responses.SubAccount{
		Object:          "sub_account",
		ID:              uuid.MustParse(FixtureSubAccountID),
		ParentAccountID: uuid.MustParse(FixtureParentAccountID),
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Name:            "Acme Subsidiary",
		Website:         "https://acme.example",
		Status:          "active",
		MonthlyCredit:   500,
		DomainCount:     3,
		MemberCount:     7,
		LastActivityAt:  &lastActivity,
	}
}

// NewSubAccountUsage returns the June 2026 usage of the parent account and
// one sub-account, with removedReception messages of removed sub-accounts.
func NewSubAccountUsage(removedReception int64) *responses.SubAccountUsageResponse {
	parentID := uuid.MustParse(FixtureParentAccountID)
	subID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	subName := "Acme Subsidiary"
	return &responses.SubAccountUsageResponse{
		BillingPeriod: responses.SubAccountUsageBillingPeriod{
			Start: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		},
		Currency:         "usd",
		AllocationMethod: "proportional",
		Parent: responses.SubAccountUsageBreakdown{
			AccountID:      &parentID,
			ReceptionCount: 1000,
			AllocatedCost:  10,
		},
		SubAccounts: []responses.SubAccountUsageBreakdown{
			{
				AccountID:      &subID,
				Name:           &subName,
				ReceptionCount: 3000,
				AllocatedCost:  30,
			},
		},
		RemovedSubAccounts: responses.SubAccountUsageBreakdown{
			ReceptionCount: removedReception,
			AllocatedCost:  5,
		},
		Total: responses.SubAccountUsageBreakdown{
			ReceptionCount: 4000,
			AllocatedCost:  40,
		},
	}
}

// NewAPIKey returns an API key of the sub-account of NewSubAccount, without
// scopes, IP allow list or secret key.
func NewAPIKey() *responses.APIKey {
	return &responses.APIKey{
		Object:    "api_key",
		ID:        uuid.MustParse(FixtureAPIKeyID),
		AccountID: uuid.MustParse(FixtureSubAccountID),
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		Label:     "Production servers",
		PublicKey: "aha-pk-test",
	}
}

// NewMessageResponse returns the response of a send to one recipient, whose
// message has the ID messageID.
func NewMessageResponse(messageID string) *responses.CreateMessageResponse {
	id := messageID
	return &responses.CreateMessageResponse{
		Object: "message.create",
		Data: []responses.CreateSingleMessageResponse{{
			Object:    "message",
			ID:        &id,
			Recipient: common.Recipient{Email: "recipient@example.com"},
			Status:    "sent",
		}},
	}
}

// NewDomain returns the domain example.com with a DMARC and a tracking DNS
// record.
func NewDomain() *responses.Domain {
	checked := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	return &responses.Domain{
		Object:         "domain",
		ID:             uuid.MustParse(FixtureDomainID),
		AccountID:      uuid.MustParse(FixtureSubAccountID),
		Domain:         "example.com",
		DNSValid:       true,
		CreatedAt:      checked,
		UpdatedAt:      checked,
		LastDNSCheckAt: &checked,
		DNSRecords: []responses.DNSRecord{
			{Type: "TXT", Host: "_dmarc.example.com", Content: "v=DMARC1; p=none;"},
			{Type: "CNAME", Host: "link.example.com", Content: "track.ahasend.com"},
		},
	}
}
