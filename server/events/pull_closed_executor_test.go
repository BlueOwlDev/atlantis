// Copyright 2017 HootSuite Media Inc.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// Modified hereafter by contributors to runatlantis/atlantis.
//
package events_test

import (
	"errors"
	"testing"

	. "github.com/petergtz/pegomock"
	"github.com/runatlantis/atlantis/server/events"
	lockmocks "github.com/runatlantis/atlantis/server/events/locking/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks"
	"github.com/runatlantis/atlantis/server/events/mocks/matchers"
	"github.com/runatlantis/atlantis/server/events/models"
	"github.com/runatlantis/atlantis/server/events/models/fixtures"
	vcsmocks "github.com/runatlantis/atlantis/server/events/vcs/mocks"
	. "github.com/runatlantis/atlantis/testing"
)

func TestCleanUpPullWorkspaceErr(t *testing.T) {
	t.Log("when workspace.Delete returns an error, we return it")
	RegisterMockTestingT(t)
	w := mocks.NewMockAtlantisWorkspace()
	pce := events.PullClosedExecutor{
		Workspace: w,
	}
	err := errors.New("err")
	When(w.Delete(fixtures.GithubRepo, fixtures.Pull)).ThenReturn(err)
	actualErr := pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
	Equals(t, "cleaning workspace: err", actualErr.Error())
}

func TestCleanUpPullUnlockErr(t *testing.T) {
	t.Log("when locker.UnlockByPull returns an error, we return it")
	RegisterMockTestingT(t)
	w := mocks.NewMockAtlantisWorkspace()
	l := lockmocks.NewMockLocker()
	pce := events.PullClosedExecutor{
		Locker:    l,
		Workspace: w,
	}
	err := errors.New("err")
	When(l.UnlockByPull(fixtures.GithubRepo.FullName, fixtures.Pull.Num)).ThenReturn(nil, err)
	actualErr := pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
	Equals(t, "cleaning up locks: err", actualErr.Error())
}

func TestCleanUpPullNoLocks(t *testing.T) {
	t.Log("when there are no locks to clean up, we don't comment")
	RegisterMockTestingT(t)
	w := mocks.NewMockAtlantisWorkspace()
	l := lockmocks.NewMockLocker()
	cp := vcsmocks.NewMockClientProxy()
	pce := events.PullClosedExecutor{
		Locker:    l,
		VCSClient: cp,
		Workspace: w,
	}
	When(l.UnlockByPull(fixtures.GithubRepo.FullName, fixtures.Pull.Num)).ThenReturn(nil, nil)
	err := pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
	Ok(t, err)
	cp.VerifyWasCalled(Never()).CreateComment(matchers.AnyModelsRepo(), AnyInt(), AnyString())
}

func TestCleanUpPullComments(t *testing.T) {
	t.Log("should comment correctly")
	RegisterMockTestingT(t)
	cases := []struct {
		Description string
		Locks       []models.ProjectLock
		Exp         string
	}{
		{
			"single lock, empty path",
			[]models.ProjectLock{
				{
					Project:   models.NewProject("owner/repo", ""),
					Workspace: "default",
				},
			},
			"- path: `owner/repo/.` workspace: `default`",
		},
		{
			"single lock, non-empty path",
			[]models.ProjectLock{
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "default",
				},
			},
			"- path: `owner/repo/path` workspace: `default`",
		},
		{
			"single path, multiple workspaces",
			[]models.ProjectLock{
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "workspace1",
				},
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "workspace2",
				},
			},
			"- path: `owner/repo/path` workspaces: `workspace1`, `workspace2`",
		},
		{
			"multiple paths, multiple workspaces",
			[]models.ProjectLock{
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "workspace1",
				},
				{
					Project:   models.NewProject("owner/repo", "path"),
					Workspace: "workspace2",
				},
				{
					Project:   models.NewProject("owner/repo", "path2"),
					Workspace: "workspace1",
				},
				{
					Project:   models.NewProject("owner/repo", "path2"),
					Workspace: "workspace2",
				},
			},
			"- path: `owner/repo/path` workspaces: `workspace1`, `workspace2`\n- path: `owner/repo/path2` workspaces: `workspace1`, `workspace2`",
		},
	}
	for _, c := range cases {
		w := mocks.NewMockAtlantisWorkspace()
		cp := vcsmocks.NewMockClientProxy()
		l := lockmocks.NewMockLocker()
		pce := events.PullClosedExecutor{
			Locker:    l,
			VCSClient: cp,
			Workspace: w,
		}
		t.Log("testing: " + c.Description)
		When(l.UnlockByPull(fixtures.GithubRepo.FullName, fixtures.Pull.Num)).ThenReturn(c.Locks, nil)
		err := pce.CleanUpPull(fixtures.GithubRepo, fixtures.Pull)
		Ok(t, err)
		_, _, comment := cp.VerifyWasCalledOnce().CreateComment(matchers.AnyModelsRepo(), AnyInt(), AnyString()).GetCapturedArguments()

		expected := "Locks and plans deleted for the projects and workspaces modified in this pull request:\n\n" + c.Exp
		Equals(t, expected, comment)
	}
}
