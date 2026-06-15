package testsuite

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var markdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

func TestNautilusDocumentationArtifactsCoverEpic13(t *testing.T) {
	docs := map[string][]string{
		"../docs/README.md": {
			"# Documentation",
			"Getting Started",
			"Module Guide",
			"Reliable Quant Program Guide",
		},
		"../docs/getting-started.md": {
			"# Getting Started",
			"Choose The Right Layer",
			"Run The Strategy In A Backtest",
		},
		"../docs/module-guide.md": {
			"# Module Guide",
			"`model`",
			"`platform`",
			"`testsuite`",
		},
		"../docs/runtime-flow.md": {
			"# Runtime Flow",
			"Market Data Flow",
			"Order Flow",
			"Reconciliation Flow",
		},
		"../docs/architecture.md": {
			"# Project Architecture",
			"Strategy Order Flow",
			"Reconciliation Flow",
			"`platform.Node`",
		},
		"../docs/guides/master-scorecard.md": {
			"# Master Scorecard",
			"Complete Feature Matrix",
			"Complete Quality Gate",
		},
		"../docs/guides/quant-use-cases.md": {
			"# Quant Developer Use Cases",
			"Order Book Imbalance Strategy",
			"Deterministic Backtest",
			"Adapter Capability-Aware Live Setup",
		},
		"../docs/guides/reliable-quant-program.md": {
			"# Reliable Quant Program Guide",
			"Create Orders Through `OrderFactory`",
			"Production Rollout Checklist",
		},
		"../docs/guides/strategy-authoring-bracket.md": {
			"# Strategy Authoring With Brackets",
			"SubmitOrderList",
			"Runtime Semantics",
		},
		"../docs/guides/live-node-configuration.md": {
			"# Live Node Configuration",
			"NodeBuilder",
			"shutdown",
		},
		"../docs/guides/reconciliation-states.md": {
			"# Reconciliation States",
			"unresolved discrepancy",
			"AuditTrail",
		},
		"../docs/guides/adapter-capability-policy.md": {
			"# Adapter Capability Policy",
			"resubscribe",
			"live write",
		},
		"../docs/guides/workflow-recipes.md": {
			"# Workflow Recipes",
			"Bracket Strategy",
			"Portfolio Query",
			"Risk Rejection",
			"Backtest Run",
			"Live Node Assembly",
		},
	}
	for path, needles := range docs {
		contentBytes, err := os.ReadFile(path)
		require.NoError(t, err, path)
		content := string(contentBytes)
		for _, needle := range needles {
			require.Contains(t, content, needle, path)
		}
	}
}

func TestChineseDocumentationArtifactsCoverEpic13(t *testing.T) {
	docs := map[string][]string{
		"../README_CN.md": {
			"# exchanges 中文文档",
			"新人从这里开始",
			"完整功能矩阵",
		},
		"../docs/README_CN.md": {
			"# 文档",
			"快速开始",
			"模块指南",
			"可靠量化程序指南",
		},
		"../docs/getting-started_CN.md": {
			"# 快速开始",
			"选择正确层级",
			"在 Backtest 中运行策略",
		},
		"../docs/module-guide_CN.md": {
			"# 模块指南",
			"`model`",
			"`platform`",
			"`testsuite`",
		},
		"../docs/runtime-flow_CN.md": {
			"# 运行时流程",
			"行情流程",
			"订单流",
			"Reconciliation 流程",
		},
		"../docs/architecture_CN.md": {
			"# 项目架构",
			"Strategy Order Flow",
			"Reconciliation Flow",
			"platform.Node",
		},
		"../docs/guides/master-scorecard_CN.md": {
			"# 主评分卡",
			"完整功能矩阵",
			"完整质量门",
		},
		"../docs/guides/quant-use-cases_CN.md": {
			"# 量化开发者使用场景",
			"Use Case 1: Order Book Imbalance Strategy",
			"Use Case 3: Deterministic Backtest",
			"Use Case 5: Adapter Capability-Aware Live Setup",
		},
		"../docs/guides/reliable-quant-program_CN.md": {
			"# 可靠量化程序指南",
			"使用 `OrderFactory` 创建订单",
			"Production Rollout Checklist",
		},
		"../docs/guides/strategy-authoring-bracket_CN.md": {
			"# Bracket 策略编写",
			"SubmitOrderList",
			"Runtime Semantics",
		},
		"../docs/guides/live-node-configuration_CN.md": {
			"# Live Node 配置",
			"NodeBuilder",
			"shutdown",
		},
		"../docs/guides/reconciliation-states_CN.md": {
			"# Reconciliation States",
			"unresolved discrepancy",
			"AuditTrail",
		},
		"../docs/guides/adapter-capability-policy_CN.md": {
			"# Adapter 能力策略",
			"resubscribe",
			"live write",
		},
		"../docs/guides/workflow-recipes_CN.md": {
			"# 工作流示例",
			"Bracket Strategy",
			"Portfolio Query",
			"Risk Rejection",
			"Backtest Run",
			"Live Node Assembly",
		},
	}
	for path, needles := range docs {
		contentBytes, err := os.ReadFile(path)
		require.NoError(t, err, path)
		content := string(contentBytes)
		for _, needle := range needles {
			require.Contains(t, content, needle, path)
		}
	}
}

func TestDocumentationHasChineseCounterparts(t *testing.T) {
	englishDocs := documentationFiles(t, []string{
		"../README.md",
		"../docs",
		"../examples/usage_comparison/README.md",
		"../sdk/README.md",
		"../sdk/okx/README.md",
	})

	for _, path := range englishDocs {
		if isChineseDocPath(path) {
			continue
		}
		require.FileExists(t, chineseCounterpartPath(path), path)
	}
}

func TestDocumentationLinksStayWithinSameLanguage(t *testing.T) {
	docs := documentationFiles(t, []string{
		"../README.md",
		"../README_CN.md",
		"../docs",
		"../examples/usage_comparison/README.md",
		"../examples/usage_comparison/README_CN.md",
		"../sdk/README.md",
		"../sdk/README_CN.md",
		"../sdk/okx/README.md",
		"../sdk/okx/README_CN.md",
	})

	for _, path := range docs {
		if filepath.Ext(path) != ".md" {
			continue
		}
		contentBytes, err := os.ReadFile(path)
		require.NoError(t, err, path)

		for _, match := range markdownLinkPattern.FindAllStringSubmatch(string(contentBytes), -1) {
			target := strings.TrimSpace(match[1])
			target = strings.Trim(target, "<>")
			targetPath := localMarkdownOrJSONLinkTarget(path, target)
			if targetPath == "" {
				continue
			}

			require.FileExists(t, targetPath, "%s links to %s", path, target)
			sourceChinese := isChineseDocPath(path)
			targetChinese := isChineseDocPath(targetPath)
			if sourceChinese {
				require.True(t, targetChinese, "%s must link to Chinese counterpart, got %s", path, target)
			} else {
				require.False(t, targetChinese, "%s must link to English counterpart, got %s", path, target)
			}
		}
	}
}

func documentationFiles(t *testing.T, roots []string) []string {
	t.Helper()

	var paths []string
	for _, root := range roots {
		info, err := os.Stat(root)
		require.NoError(t, err, root)
		if !info.IsDir() {
			if isDocumentationArtifact(root) {
				paths = append(paths, filepath.Clean(root))
			}
			continue
		}

		err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			require.NoError(t, err, path)
			if entry.IsDir() {
				return nil
			}
			if isDocumentationArtifact(path) {
				paths = append(paths, filepath.Clean(path))
			}
			return nil
		})
		require.NoError(t, err, root)
	}
	sort.Strings(paths)
	return paths
}

func isDocumentationArtifact(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".md" || ext == ".json"
}

func isChineseDocPath(path string) bool {
	ext := filepath.Ext(path)
	base := filepath.Base(path)
	return strings.HasSuffix(strings.TrimSuffix(base, ext), "_CN")
}

func chineseCounterpartPath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + "_CN" + ext
}

func localMarkdownOrJSONLinkTarget(sourcePath, rawTarget string) string {
	if rawTarget == "" ||
		strings.HasPrefix(rawTarget, "#") ||
		strings.Contains(rawTarget, "://") ||
		strings.HasPrefix(rawTarget, "mailto:") {
		return ""
	}
	target := strings.SplitN(rawTarget, "#", 2)[0]
	target = strings.SplitN(target, "?", 2)[0]
	ext := filepath.Ext(target)
	if ext != ".md" && ext != ".json" {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourcePath), target))
}
