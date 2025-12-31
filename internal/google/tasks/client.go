package tasks

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/option"
	"google.golang.org/api/tasks/v1"
)

type Client struct {
	svc *tasks.Service
}

func New(ctx context.Context, httpClient *http.Client) (*Client, error) {
	svc, err := tasks.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &Client{svc: svc}, nil
}

func (c *Client) CreateTask(listID string, task *tasks.Task) (*tasks.Task, error) {
	if listID == "" {
		return nil, fmt.Errorf("listID is required")
	}
	return c.svc.Tasks.Insert(listID, task).Do()
}

func (c *Client) CreateTaskWithParent(listID string, task *tasks.Task, parentID string) (*tasks.Task, error) {
	if listID == "" {
		return nil, fmt.Errorf("listID is required")
	}
	call := c.svc.Tasks.Insert(listID, task)
	if parentID != "" {
		call.Parent(parentID)
	}
	return call.Do()
}

func (c *Client) UpdateTask(listID string, task *tasks.Task) (*tasks.Task, error) {
	if listID == "" || task == nil {
		return nil, fmt.Errorf("listID and task are required")
	}
	return c.svc.Tasks.Update(listID, task.Id, task).Do()
}

func (c *Client) GetTask(listID, taskID string) (*tasks.Task, error) {
	return c.svc.Tasks.Get(listID, taskID).Do()
}

func (c *Client) CompleteTask(listID, taskID string) (*tasks.Task, error) {
	task, err := c.GetTask(listID, taskID)
	if err != nil {
		return nil, err
	}
	task.Status = "completed"
	return c.svc.Tasks.Update(listID, taskID, task).Do()
}

func (c *Client) ListTasks(listID string, showCompleted bool) ([]*tasks.Task, error) {
	call := c.svc.Tasks.List(listID)
	call.ShowCompleted(showCompleted)
	resp, err := call.Do()
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) FindTaskByTitle(listID, title string) (*tasks.Task, error) {
	items, err := c.ListTasks(listID, false)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Title == title && item.Status != "completed" {
			return item, nil
		}
	}
	return nil, fmt.Errorf("task not found")
}

func (c *Client) ListTaskLists() ([]*tasks.TaskList, error) {
	resp, err := c.svc.Tasklists.List().Do()
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) CreateTaskList(title string) (*tasks.TaskList, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	list := &tasks.TaskList{Title: title}
	return c.svc.Tasklists.Insert(list).Do()
}
