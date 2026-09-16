package api

import (
	"testing"

	"appsire-go/internal/engine"
	"appsire-go/internal/sunat"
)

func TestValidateProposalBindingRejectsChangedIdentity(t *testing.T) {
	t.Parallel()

	server := &Server{proposals: map[sunat.ProposalBook]proposalBinding{
		sunat.ProposalRCE: {RUC: "20600000001", Period: "202608", Ticket: "ticket-1"},
	}}
	base := engine.DownloadRequest{
		ProposalBook:   "RCE",
		ProposalPeriod: "202608",
		ProposalTicket: "ticket-1",
		OwnerRUC:       "20600000001",
	}
	tests := []struct {
		name       string
		request    engine.DownloadRequest
		currentRUC string
		wantError  bool
	}{
		{name: "same proposal", request: base, currentRUC: "20600000001"},
		{name: "company changed", request: base, currentRUC: "20999999999", wantError: true},
		{name: "period changed", request: withProposalPeriod(base, "202609"), currentRUC: "20600000001", wantError: true},
		{name: "ticket changed", request: withProposalTicket(base, "ticket-2"), currentRUC: "20600000001", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateProposalBinding(tt.request, tt.currentRUC)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateProposalBinding() error = %v, wantError %t", err, tt.wantError)
			}
		})
	}
}

func withProposalPeriod(request engine.DownloadRequest, period string) engine.DownloadRequest {
	request.ProposalPeriod = period
	return request
}

func withProposalTicket(request engine.DownloadRequest, ticket string) engine.DownloadRequest {
	request.ProposalTicket = ticket
	return request
}
