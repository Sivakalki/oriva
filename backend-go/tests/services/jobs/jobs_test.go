package jobs_test

import (
	"context"
	"strings"
	"testing"

	apxerrors "oriva/backend-go/errors"
	"oriva/backend-go/models/job"
	"oriva/backend-go/repositories/postgres"
	"oriva/backend-go/services/jobs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	created  *job.Job
	getErr   error
	updErr   error
	lastArgs []any
}

func (f *fakeRepo) Create(_ context.Context, org, title, desc string) (*job.Job, error) {
	f.lastArgs = []any{org, title, desc}
	return &job.Job{ID: "j1", OrgID: org, Title: title, Description: desc}, nil
}
func (f *fakeRepo) ListByOrg(context.Context, string) ([]job.Job, error) { return nil, nil }
func (f *fakeRepo) Get(context.Context, string, string) (*job.Job, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &job.Job{ID: "j1"}, nil
}
func (f *fakeRepo) Update(_ context.Context, _, _ string, t, d *string) (*job.Job, error) {
	if f.updErr != nil {
		return nil, f.updErr
	}
	return &job.Job{ID: "j1"}, nil
}

func kind(t *testing.T, err error) apxerrors.Kind {
	t.Helper()
	var ae *apxerrors.Error
	require.True(t, apxerrors.As(err, &ae))
	return ae.Kind
}

func TestCreate(t *testing.T) {
	svc := jobs.NewService(&fakeRepo{})

	_, err := svc.Create(context.Background(), "o1", jobs.CreateInput{Title: "  "})
	assert.Equal(t, apxerrors.Invalid, kind(t, err))

	_, err = svc.Create(context.Background(), "o1", jobs.CreateInput{Title: strings.Repeat("x", 201)})
	assert.Equal(t, apxerrors.Invalid, kind(t, err))

	j, err := svc.Create(context.Background(), "o1", jobs.CreateInput{Title: "Backend Eng", Description: "d"})
	require.NoError(t, err)
	assert.Equal(t, "Backend Eng", j.Title)
}

func TestUpdate_NoFields(t *testing.T) {
	_, err := jobs.NewService(&fakeRepo{}).Update(context.Background(), "o1", "j1", jobs.UpdateInput{})
	assert.Equal(t, apxerrors.Invalid, kind(t, err))
}

func TestGet_NotFound(t *testing.T) {
	svc := jobs.NewService(&fakeRepo{getErr: postgres.ErrNotFound})
	_, err := svc.Get(context.Background(), "o1", "nope")
	assert.Equal(t, apxerrors.NotFound, kind(t, err))
}

func TestUpdate_NotFound(t *testing.T) {
	svc := jobs.NewService(&fakeRepo{updErr: postgres.ErrNotFound})
	title := "New"
	_, err := svc.Update(context.Background(), "o1", "nope", jobs.UpdateInput{Title: &title})
	assert.Equal(t, apxerrors.NotFound, kind(t, err))
}
