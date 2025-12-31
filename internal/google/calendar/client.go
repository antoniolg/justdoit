package calendar

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Client struct {
	svc *calendar.Service
}

func New(ctx context.Context, httpClient *http.Client) (*Client, error) {
	svc, err := calendar.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &Client{svc: svc}, nil
}

func (c *Client) CreateEvent(calendarID string, event *calendar.Event) (*calendar.Event, error) {
	if calendarID == "" {
		return nil, fmt.Errorf("calendarID is required")
	}
	return c.svc.Events.Insert(calendarID, event).Do()
}

func (c *Client) UpdateEvent(calendarID string, event *calendar.Event) (*calendar.Event, error) {
	if calendarID == "" || event == nil {
		return nil, fmt.Errorf("calendarID and event are required")
	}
	return c.svc.Events.Update(calendarID, event.Id, event).Do()
}

func (c *Client) GetEvent(calendarID, eventID string) (*calendar.Event, error) {
	return c.svc.Events.Get(calendarID, eventID).Do()
}

func (c *Client) ListEvents(calendarID string, timeMin, timeMax string) ([]*calendar.Event, error) {
	call := c.svc.Events.List(calendarID).
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(timeMin).
		TimeMax(timeMax).
		OrderBy("startTime")
	resp, err := call.Do()
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListCalendars() ([]*calendar.CalendarListEntry, error) {
	resp, err := c.svc.CalendarList.List().Do()
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) FindEventByTaskID(calendarID, taskID string) (*calendar.Event, error) {
	q := fmt.Sprintf("justdoit_task_id=%s", taskID)
	call := c.svc.Events.List(calendarID).
		Q(q).
		ShowDeleted(false).
		SingleEvents(true)
	resp, err := call.Do()
	if err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("event not found for task %s", taskID)
	}
	return resp.Items[0], nil
}
