package lists

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testListID = "22222222-2222-4222-8222-222222222222"

func testList() *responses.ContactList {
	return &responses.ContactList{
		Object:       "list",
		ID:           uuid.MustParse(testListID),
		Name:         "Newsletter",
		Tags:         []string{"news"},
		ContactCount: 3,
	}
}

// list ------------------------------------------------------------------------

func TestListForwardsPaginationAndName(t *testing.T) {
	limit := int32(5)
	after := "next"
	name := "News"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetLists", requests.GetListsParams{Limit: &limit, After: &after, Name: &name}).
		Return(&responses.PaginatedContactListsResponse{Object: "list", Data: []responses.ContactList{*testList()}}, nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewListCommand(), "--limit", "5", "--after", after, "--name", name)
	require.NoError(t, err)
	assert.Contains(t, output, "Newsletter")
	mockClient.AssertExpectations(t)
}

func TestListValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"--after", "a", "--before", "b"}, "--after and --before cannot be used together"},
		{[]string{"--limit", "101"}, "--limit must be between 0 and 100"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewListCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

// get -------------------------------------------------------------------------

func TestGetSendsListID(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetList", uuid.MustParse(testListID)).Return(testList(), nil)
	testutil.InjectClient(t, mockClient)

	output, err := testutil.RunPlain(t, NewGetCommand(), testListID)
	require.NoError(t, err)
	assert.Contains(t, output, "Name: Newsletter")
	mockClient.AssertExpectations(t)
}

func TestGetRejectsBadListIDBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewGetCommand(), "not-a-uuid")
	require.Error(t, err)
	assert.Equal(t, `invalid list ID value "not-a-uuid": not a UUID`, err.Error())
	assert.False(t, *authCalled)
}

// create ----------------------------------------------------------------------

func TestCreateSendsNameDescriptionTagsAndIdempotencyKey(t *testing.T) {
	description := "Monthly news"
	expected := requests.CreateListRequest{
		Name:        "Newsletter",
		Description: &description,
		Tags:        []string{"news", "monthly"},
	}
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateList", expected, testutil.IdempotencyKeyArg("list-key")).Return(testList(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Newsletter", "--description", description,
		"--tag", "news", "--tag", "monthly", "--idempotency-key", "list-key")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateSplitsTagsOnCommas(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateList", requests.CreateListRequest{Name: "Newsletter", Tags: []string{"news", "weekly", "digest"}}).
		Return(testList(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Newsletter", "--tag", "news,weekly", "--tag", "digest")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateEmptyTagRejectedBeforeAuthentication(t *testing.T) {
	for _, value := range []string{"", "news,,weekly"} {
		t.Run(value, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)

			_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Newsletter", "--tag", value)
			require.Error(t, err)
			assert.Equal(t, "--tag cannot be empty", err.Error())
			assert.False(t, *authCalled)
		})
	}
}

func TestCreateWithoutTagsLeavesTagsNil(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateList", requests.CreateListRequest{Name: "Newsletter"}).Return(testList(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--name", "Newsletter")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateWithoutNameRejectedBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewCreateCommand(), "--tag", "news")
	require.Error(t, err)
	assert.Equal(t, "--name is required", err.Error())
	assert.False(t, *authCalled)
}

// update ----------------------------------------------------------------------

func TestUpdateSendsOnlyChangedFields(t *testing.T) {
	description := ""
	tags := []string{"news", "weekly"}
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateList", uuid.MustParse(testListID),
		requests.UpdateListRequest{Description: &description, Tags: &tags}).
		Return(testList(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testListID, "--description", "", "--tag", "news", "--tag", "weekly")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateSendsName(t *testing.T) {
	name := "Weekly"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateList", uuid.MustParse(testListID), requests.UpdateListRequest{Name: &name}).
		Return(testList(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testListID, "--name", name)
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateClearTagsSendsEmptySlice(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateList", uuid.MustParse(testListID), mock.MatchedBy(func(req requests.UpdateListRequest) bool {
		return req.Name == nil && req.Description == nil && req.Tags != nil && len(*req.Tags) == 0
	})).Return(testList(), nil)
	testutil.InjectClient(t, mockClient)

	_, err := testutil.RunPlain(t, NewUpdateCommand(), testListID, "--clear-tags")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestUpdateValidationHappensBeforeAuthentication(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{testListID, "--tag", "news", "--clear-tags"}, "--tag and --clear-tags cannot be used together"},
		{[]string{testListID, "--tag", ""}, "--tag cannot be empty; use --clear-tags to clear it"},
		{[]string{testListID, "--tag", "news,"}, "--tag cannot be empty; use --clear-tags to clear it"},
		{[]string{testListID}, "at least one of --name, --description, --tag or --clear-tags must be provided"},
		{[]string{testListID, "--clear-tags=false"}, "at least one of --name, --description, --tag or --clear-tags must be provided"},
		{[]string{"not-a-uuid", "--name", "x"}, `invalid list ID value "not-a-uuid": not a UUID`},
		{[]string{testListID, "--name", ""}, "--name cannot be empty"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, NewUpdateCommand(), test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

// delete ----------------------------------------------------------------------

func TestDeleteDeclinedMakesNoCall(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", testListID)
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete list "+testListID)
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedPromptsOnStderrOnly(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteList", uuid.MustParse(testListID)).Return(&common.SuccessResponse{Message: "List deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", " YES \n", testListID)
	require.NoError(t, err)
	assert.Contains(t, stderr, "Are you sure")
	assert.Equal(t, "List deleted\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteForcedSkipsPrompt(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteList", uuid.MustParse(testListID)).Return(&common.SuccessResponse{}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "", testListID, "--force")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Equal(t, "List '"+testListID+"' deleted successfully\n", stdout)
	mockClient.AssertExpectations(t)
}

func TestDeleteRejectsBadListIDBeforePrompt(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "not-a-uuid")
	require.Error(t, err)
	assert.NotContains(t, stderr, "Are you sure")
	assert.False(t, *authCalled)
}
