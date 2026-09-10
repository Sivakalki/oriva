package candidates

import (
	"context"
	"testing"

	"oriva/backend-go/db/postgres"
	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/candidate"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	createErr error
	gotEmail  string
}

func (f *fakeRepo) Create(_ context.Context, org, email, name, resume string) (*candidate.Candidate, error) {
	f.gotEmail = email
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &candidate.Candidate{ID: "c1", OrgID: org, Email: email, Name: name}, nil
}
func (f *fakeRepo) ListByOrg(context.Context, string) ([]candidate.Candidate, error) { return nil, nil }
func (f *fakeRepo) Get(context.Context, string, string) (*candidate.Candidate, error) {
	return nil, postgres.ErrNotFound
}
func (f *fakeRepo) Update(context.Context, string, string, *string, *string) (*candidate.Candidate, error) {
	return &candidate.Candidate{ID: "c1"}, nil
}

func kind(t *testing.T, err error) apxerrors.Kind {
	t.Helper()
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	return ae.Kind
}

func TestCreate_BadEmail(t *testing.T) {
	_, err := NewService(&fakeRepo{}).Create(context.Background(), "o1", CreateInput{Email: "nope", Name: "A"})
	assert.Equal(t, apxerrors.Invalid, kind(t, err))
}

func TestCreate_Conflict(t *testing.T) {
	svc := NewService(&fakeRepo{createErr: postgres.ErrConflict})
	_, err := svc.Create(context.Background(), "o1", CreateInput{Email: "a@b.com", Name: "A"})
	assert.Equal(t, apxerrors.Conflict, kind(t, err))
}

func TestCreate_Happy(t *testing.T) {
	f := &fakeRepo{}
	c, err := NewService(f).Create(context.Background(), "o1", CreateInput{Email: "  A@B.com ", Name: " Alice "})
	require.NoError(t, err)
	assert.Equal(t, "Alice", c.Name)
	assert.Equal(t, "A@B.com", f.gotEmail) // normalized by mail.ParseAddress, case preserved
}

func TestGet_NotFound(t *testing.T) {
	_, err := NewService(&fakeRepo{}).Get(context.Background(), "o1", "x")
	assert.Equal(t, apxerrors.NotFound, kind(t, err))
}
