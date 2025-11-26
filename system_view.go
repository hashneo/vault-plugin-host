// Copyright 2025 vault-plugin-host Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/vault/sdk/helper/consts"
	"github.com/hashicorp/vault/sdk/helper/license"
	"github.com/hashicorp/vault/sdk/helper/pluginutil"
	"github.com/hashicorp/vault/sdk/helper/wrapping"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/sdk/rotation"
)

// TestSystemView implements logical.SystemView with stub implementations
type TestSystemView struct {
	logical.SystemView
}

func (s *TestSystemView) DefaultLeaseTTL() time.Duration                     { return 30 * time.Second }
func (s *TestSystemView) MaxLeaseTTL() time.Duration                         { return 60 * time.Minute }
func (s *TestSystemView) SudoPrivilege(context.Context, string, string) bool { return false }
func (s *TestSystemView) Tainted() bool                                      { return false }
func (s *TestSystemView) CachingDisabled() bool                              { return false }
func (s *TestSystemView) LocalMount() bool                                   { return false }
func (s *TestSystemView) MlockEnabled() bool                                 { return false }
func (s *TestSystemView) ReplicationState() consts.ReplicationState          { return consts.ReplicationUnknown }
func (s *TestSystemView) HasFeature(feature license.Features) bool           { return false }

func (s *TestSystemView) ResponseWrapData(ctx context.Context, data map[string]interface{}, ttl time.Duration, jwt bool) (*wrapping.ResponseWrapInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TestSystemView) LookupPlugin(ctx context.Context, name string, pluginType consts.PluginType) (*pluginutil.PluginRunner, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TestSystemView) LookupPluginVersion(ctx context.Context, pluginName string, pluginType consts.PluginType, version string) (*pluginutil.PluginRunner, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TestSystemView) ListVersionedPlugins(ctx context.Context, pluginType consts.PluginType) ([]pluginutil.VersionedPlugin, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TestSystemView) EntityInfo(entityID string) (*logical.Entity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TestSystemView) GroupsForEntity(entityID string) ([]*logical.Group, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TestSystemView) PluginEnv(ctx context.Context) (*logical.PluginEnvironment, error) {
	return &logical.PluginEnvironment{}, nil
}

func (s *TestSystemView) GeneratePasswordFromPolicy(ctx context.Context, policyName string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (s *TestSystemView) ClusterID(ctx context.Context) (string, error) {
	return "test-cluster", nil
}

func (s *TestSystemView) NewPluginClient(ctx context.Context, config pluginutil.PluginClientConfig) (pluginutil.PluginClient, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TestSystemView) VaultVersion(ctx context.Context) (string, error) {
	return "test-version", nil
}

func (s *TestSystemView) DeregisterRotationJob(ctx context.Context, req *rotation.RotationJobDeregisterRequest) error {
	return fmt.Errorf("not implemented")
}

func (s *TestSystemView) GetRotationInformation(ctx context.Context, req *rotation.RotationInfoRequest) (*rotation.RotationInfoResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *TestSystemView) RegisterRotationJob(ctx context.Context, req *rotation.RotationJobConfigureRequest) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (s *TestSystemView) DownloadExtractVerifyPlugin(ctx context.Context, runner *pluginutil.PluginRunner) error {
	return fmt.Errorf("not implemented")
}

func (s *TestSystemView) GenerateIdentityToken(ctx context.Context, req *pluginutil.IdentityTokenRequest) (*pluginutil.IdentityTokenResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
