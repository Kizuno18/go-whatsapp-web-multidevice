package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow/types"
)

func TestRegisteredUserCheck(t *testing.T) {
	phone := "+5511987654321"
	for _, tc := range []struct {
		name string
		rows []types.IsOnWhatsAppResponse
		err  error
		want string
		fail bool
	}{
		{name: "canonical phone", rows: []types.IsOnWhatsAppResponse{{Query: phone, IsIn: true, JID: types.NewJID("551187654321", types.DefaultUserServer)}}, want: "551187654321@s.whatsapp.net"},
		{name: "phone behind lid", rows: []types.IsOnWhatsAppResponse{{Query: phone, IsIn: true, JID: types.NewJID("123", types.HiddenUserServer), PhoneNumber: types.NewJID("551187654321", types.DefaultUserServer)}}, want: "551187654321@s.whatsapp.net"},
		{name: "unregistered", rows: []types.IsOnWhatsAppResponse{{Query: phone}}},
		{name: "missing", fail: true},
		{name: "provider error", err: errors.New("provider unavailable"), fail: true},
		{name: "wrong query", rows: []types.IsOnWhatsAppResponse{{Query: "+551100000000", IsIn: true}}, fail: true},
		{name: "missing phone jid", rows: []types.IsOnWhatsAppResponse{{Query: phone, IsIn: true, JID: types.NewJID("123", types.HiddenUserServer)}}, fail: true},
		{name: "multiple", rows: []types.IsOnWhatsAppResponse{{Query: phone}, {Query: phone}}, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := checkRegisteredUser(context.Background(), phone, func(_ context.Context, phones []string) ([]types.IsOnWhatsAppResponse, error) {
				require.Equal(t, []string{phone}, phones)
				return tc.rows, tc.err
			})
			if tc.fail {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got.RegisteredJID)
			require.Equal(t, tc.want != "", got.IsOnWhatsApp)
		})
	}
}
