package group_service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	instance_model "whatsgo/pkg/instance/model"
	logger_wrapper "whatsgo/pkg/logger"
	"whatsgo/pkg/utils"
	whatsmeow_service "whatsgo/pkg/whatsmeow/service"
	"github.com/gin-gonic/gin"
	"github.com/vincent-petithory/dataurl"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type GroupService interface {
	ListGroups(instance *instance_model.Instance) ([]*types.GroupInfo, error)
	GetGroupInfo(data *GetGroupInfoStruct, instance *instance_model.Instance) (*types.GroupInfo, error)
	GetGroupInviteLink(data *GetGroupInviteLinkStruct, instance *instance_model.Instance) (string, error)
	GetGroupInfoFromInviteLink(data *GetGroupInfoFromInviteLinkStruct, instance *instance_model.Instance) (*types.GroupInfo, error)
	SetGroupPhoto(data *SetGroupPhotoStruct, instance *instance_model.Instance) (string, error)
	SetGroupName(data *SetGroupNameStruct, instance *instance_model.Instance) error
	SetGroupDescription(data *SetGroupDescriptionStruct, instance *instance_model.Instance) error
	SetGroupAnnounce(data *SetGroupAnnounceStruct, instance *instance_model.Instance) error
	CreateGroup(data *CreateGroupStruct, instance *instance_model.Instance) (gin.H, error)
	UpdateParticipant(data *AddParticipantStruct, instance *instance_model.Instance) error
	GetMyGroups(instance *instance_model.Instance) ([]types.GroupInfo, error)
	JoinGroupLink(data *JoinGroupStruct, instance *instance_model.Instance) error
	LeaveGroup(data *LeaveGroupStruct, instance *instance_model.Instance) error
}

type groupService struct {
	clientPointer    map[string]*whatsmeow.Client
	whatsmeowService whatsmeow_service.WhatsmeowService
	loggerWrapper    *logger_wrapper.LoggerManager
}

type SimpleGroupInfo struct {
	JID       types.JID `json:"jid"`
	GroupName string    `json:"groupName"`
}

type GroupCollection struct {
	Groups []SimpleGroupInfo
}

type GetGroupInfoStruct struct {
	GroupJID string `json:"groupJid"`
}

type GetGroupInviteLinkStruct struct {
	GroupJID string `json:"groupJid"`
	Reset    bool   `json:"reset"`
}

type GetGroupInfoFromInviteLinkStruct struct {
	Code string `json:"code"`
}

type SetGroupPhotoStruct struct {
	GroupJID string `json:"groupJid"`
	Image    string `json:"image"`
}

type SetGroupNameStruct struct {
	GroupJID string `json:"groupJid"`
	Name     string `json:"name"`
}

type SetGroupDescriptionStruct struct {
	GroupJID    string `json:"groupJid"`
	Description string `json:"description"`
}

type SetGroupAnnounceStruct struct {
	GroupJID string `json:"groupJid"`
	Announce bool   `json:"announce"`
}

type CreateGroupStruct struct {
	GroupName    string   `json:"groupName"`
	Participants []string `json:"participants"`
}

type AddParticipantStruct struct {
	GroupJID     types.JID                   `json:"groupJid"`
	Participants []string                    `json:"participants"`
	Action       whatsmeow.ParticipantChange `json:"action"`
}

type JoinGroupStruct struct {
	Code string `json:"code"`
}

type LeaveGroupStruct struct {
	GroupJID types.JID `json:"groupJid"`
}

func (g *groupService) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
	client := g.clientPointer[instanceId]
	g.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		g.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := g.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			g.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		g.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance started, waiting 2 seconds...", instanceId)
		time.Sleep(2 * time.Second)

		client = g.clientPointer[instanceId]
		g.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceId,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			g.loggerWrapper.GetLogger(instanceId).LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceId,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		g.loggerWrapper.GetLogger(instanceId).LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceId,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	g.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Client successfully validated - Connected: %v", instanceId, client.IsConnected())
	return client, nil
}

func (g *groupService) ListGroups(instance *instance_model.Instance) ([]*types.GroupInfo, error) {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	resp, err := client.GetJoinedGroups(context.Background())
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error getting groups: %v", instance.Id, err)
		return nil, err
	}

	gc := new(GroupCollection)
	for _, info := range resp {
		simpleGroup := SimpleGroupInfo{
			JID:       info.JID,
			GroupName: info.GroupName.Name,
		}
		gc.Groups = append(gc.Groups, simpleGroup)
	}

	return resp, nil
}

func (g *groupService) GetGroupInfo(data *GetGroupInfoStruct, instance *instance_model.Instance) (*types.GroupInfo, error) {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	recipient, ok := utils.ParseJID(data.GroupJID)
	if !ok {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return nil, errors.New("invalid group jid")
	}

	resp, err := client.GetGroupInfo(context.Background(), recipient)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error mute chat: %v", instance.Id, err)
		return nil, err
	}

	return resp, nil
}

func (g *groupService) GetGroupInviteLink(data *GetGroupInviteLinkStruct, instance *instance_model.Instance) (string, error) {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	recipient, ok := utils.ParseJID(data.GroupJID)
	if !ok {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid group jid")
	}

	resp, err := client.GetGroupInviteLink(context.Background(), recipient, data.Reset)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error mute chat: %v", instance.Id, err)
		return "", err
	}

	return resp, nil
}

func (g *groupService) GetGroupInfoFromInviteLink(data *GetGroupInfoFromInviteLinkStruct, instance *instance_model.Instance) (*types.GroupInfo, error) {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	code := data.Code
	if strings.Contains(code, "chat.whatsapp.com/") {
		parts := strings.Split(code, "chat.whatsapp.com/")
		if len(parts) > 1 {
			code = parts[1]
		}
	}

	resp, err := client.GetGroupInfoFromLink(context.Background(), code)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error getting group info from link: %v", instance.Id, err)
		return nil, err
	}

	return resp, nil
}

func (g *groupService) SetGroupPhoto(data *SetGroupPhotoStruct, instance *instance_model.Instance) (string, error) {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return "", err
	}

	recipient, ok := utils.ParseJID(data.GroupJID)
	if !ok {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return "", errors.New("invalid group jid")
	}

	var fileData []byte

	if strings.HasPrefix(data.Image, "http://") || strings.HasPrefix(data.Image, "https://") {
		resp, err := http.Get(data.Image)
		if err != nil {
			g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Could not download image from URL", instance.Id)
			return "", fmt.Errorf("failed to fetch image from URL: %v", err)
		}
		defer resp.Body.Close()

		fileData, err = io.ReadAll(resp.Body)
		if err != nil {
			g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Could not read image data from URL", instance.Id)
			return "", fmt.Errorf("failed to read image data: %v", err)
		}

	} else if strings.HasPrefix(data.Image, "data:image/jpeg;base64,") || strings.HasPrefix(data.Image, "data:image/png;base64,") {
		dataURL, err := dataurl.DecodeString(data.Image)
		if err != nil {
			g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Could not decode base64 encoded data from payload", instance.Id)
			return "", err
		}
		fileData = dataURL.Data
	} else {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Image data should start with \"data:image/jpeg;base64,\" or be a valid URL", instance.Id)
		return "", errors.New("image data should be a valid URL or start with \"data:image/jpeg;base64,\"")
	}

	pictureID, err := client.SetGroupPhoto(context.Background(), recipient, fileData)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error setting group photo: %v", instance.Id, err)
		return "", err
	}

	return pictureID, nil
}

func (g *groupService) SetGroupName(data *SetGroupNameStruct, instance *instance_model.Instance) error {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	recipient, ok := utils.ParseJID(data.GroupJID)
	if !ok {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return errors.New("invalid group jid")
	}

	err = client.SetGroupName(context.Background(), recipient, data.Name)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error mute chat: %v", instance.Id, err)
		return err
	}

	return nil
}

func (g *groupService) SetGroupDescription(data *SetGroupDescriptionStruct, instance *instance_model.Instance) error {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	recipient, ok := utils.ParseJID(data.GroupJID)
	if !ok {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return errors.New("invalid group jid")
	}

	err = client.SetGroupDescription(context.Background(), recipient, data.Description)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error mute chat: %v", instance.Id, err)
		return err
	}

	return nil
}

func (g *groupService) CreateGroup(data *CreateGroupStruct, instance *instance_model.Instance) (gin.H, error) {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	var participants []types.JID
	for _, participant := range data.Participants {
		recipient, ok := utils.ParseJID(participant)
		participants = append(participants, recipient)
		if !ok {
			g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
			return nil, errors.New("invalid phone number")
		}
	}

	resp, err := client.CreateGroup(context.Background(), whatsmeow.ReqCreateGroup{
		Name:         data.GroupName,
		Participants: participants,
	})
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error create group: %v", instance.Id, err)
		return nil, err
	}

	var failed []types.JID
	for _, participant := range resp.Participants {
		if participant.Error != 0 {
			failed = append(failed, participant.JID)
		}
	}

	var added []types.JID
	infoResp, err := client.GetGroupInfo(context.Background(), resp.JID)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error get group info: %v", instance.Id, err)
		return nil, err
	}
	for _, add := range infoResp.Participants {
		added = append(added, add.JID)
	}

	response := gin.H{
		"jid":    resp.JID,
		"name":   resp.Name,
		"owner":  resp.OwnerJID,
		"added":  added,
		"failed": failed,
	}

	return response, nil
}

func (g *groupService) UpdateParticipant(data *AddParticipantStruct, instance *instance_model.Instance) error {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	var participants []types.JID
	for _, participant := range data.Participants {
		recipient, ok := utils.ParseJID(participant)
		participants = append(participants, recipient)
		if !ok {
			g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
			return errors.New("invalid phone number")
		}
	}

	_, err = client.UpdateGroupParticipants(context.Background(), data.GroupJID, participants, data.Action)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error create group: %v", instance.Id, err)
		return err
	}

	return nil
}

func (g *groupService) GetMyGroups(instance *instance_model.Instance) ([]types.GroupInfo, error) {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	resp, err := client.GetJoinedGroups(context.Background())
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error create group: %v", instance.Id, err)
		return nil, err
	}

	var jid string = client.Store.ID.String()
	var jidClear = strings.Split(jid, ".")[0]
	jidOfAdmin, ok := utils.ParseJID(jidClear)
	if !ok {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return nil, errors.New("invalid phone number")
	}
	var adminGroups []types.GroupInfo
	for _, group := range resp {
		if group.OwnerJID == jidOfAdmin {
			adminGroups = append(adminGroups, *group)
			_ = adminGroups
		}
	}

	return adminGroups, nil
}

func (g *groupService) JoinGroupLink(data *JoinGroupStruct, instance *instance_model.Instance) error {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	_, err = client.JoinGroupWithLink(context.Background(), data.Code)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error create group: %v", instance.Id, err)
		return err
	}

	return nil
}

func (g *groupService) LeaveGroup(data *LeaveGroupStruct, instance *instance_model.Instance) error {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	err = client.LeaveGroup(context.Background(), data.GroupJID)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error leave group: %v", instance.Id, err)
		return err
	}

	return nil
}

func (g *groupService) SetGroupAnnounce(data *SetGroupAnnounceStruct, instance *instance_model.Instance) error {
	client, err := g.ensureClientConnected(instance.Id)
	if err != nil {
		return err
	}

	recipient, ok := utils.ParseJID(data.GroupJID)
	if !ok {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error validating message fields", instance.Id)
		return errors.New("invalid group jid")
	}

	err = client.SetGroupAnnounce(context.Background(), recipient, data.Announce)
	if err != nil {
		g.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error set group announce: %v", instance.Id, err)
		return err
	}

	return nil
}

func NewGroupService(
	clientPointer map[string]*whatsmeow.Client,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	loggerWrapper *logger_wrapper.LoggerManager,
) GroupService {
	return &groupService{
		clientPointer:    clientPointer,
		whatsmeowService: whatsmeowService,
		loggerWrapper:    loggerWrapper,
	}
}
