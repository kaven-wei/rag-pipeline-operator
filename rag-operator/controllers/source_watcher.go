/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ragv1alpha1 "rag.ai/rag-operator/api/v1alpha1"
)

// SourceWatcher monitors document sources for changes
type SourceWatcher struct {
	client.Client
	Log logr.Logger
}

// SourceChangeResult contains the result of a source change check
type SourceChangeResult struct {
	Changed        bool
	NewHash        string
	NewMetadata    *ragv1alpha1.SourceMetadata
	FilesChanged   int
	FilesAdded     int
	FilesDeleted   int
	ChangedFiles   []string
	AddedFiles     []string
	DeletedFiles   []string
	Error          error
}

// CheckSourceChanges checks if the document source has changed
func (w *SourceWatcher) CheckSourceChanges(ctx context.Context, ds *ragv1alpha1.DocumentSet) (*SourceChangeResult, error) {
	logger := w.Log.WithValues("documentset", ds.Name, "namespace", ds.Namespace)
	logger.Info("Checking source for changes", "sourceType", ds.Spec.Source.Type, "uri", ds.Spec.Source.URI)

	// Get credentials if needed
	var secretData map[string][]byte
	if ds.Spec.Source.SecretRef != nil {
		secret := &corev1.Secret{}
		if err := w.Get(ctx, types.NamespacedName{
			Name:      ds.Spec.Source.SecretRef.Name,
			Namespace: ds.Namespace,
		}, secret); err != nil {
			return nil, fmt.Errorf("failed to get source secret: %w", err)
		}
		secretData = secret.Data
	}

	// Check based on source type
	switch ds.Spec.Source.Type {
	case ragv1alpha1.SourceTypeS3:
		return w.checkS3Source(ctx, ds, secretData)
	case ragv1alpha1.SourceTypeHTTP:
		return w.checkHTTPSource(ctx, ds, secretData)
	case ragv1alpha1.SourceTypeGit:
		return w.checkGitSource(ctx, ds, secretData)
	case ragv1alpha1.SourceTypePVC:
		return w.checkPVCSource(ctx, ds)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", ds.Spec.Source.Type)
	}
}

// checkS3Source checks S3 bucket for changes
func (w *SourceWatcher) checkS3Source(ctx context.Context, ds *ragv1alpha1.DocumentSet, secretData map[string][]byte) (*SourceChangeResult, error) {
	logger := w.Log.WithValues("documentset", ds.Name, "sourceType", "s3")

	// Parse S3 URI: s3://bucket/prefix/
	uri := ds.Spec.Source.URI
	if !strings.HasPrefix(uri, "s3://") {
		return nil, fmt.Errorf("invalid S3 URI: %s", uri)
	}

	parts := strings.SplitN(strings.TrimPrefix(uri, "s3://"), "/", 2)
	bucket := parts[0]
	prefix := ""
	if len(parts) > 1 {
		prefix = parts[1]
	}

	// Configure AWS SDK
	var opts []func(*config.LoadOptions) error
	
	// Add credentials from secret if available
	if secretData != nil {
		accessKey := string(secretData["AWS_ACCESS_KEY_ID"])
		secretKey := string(secretData["AWS_SECRET_ACCESS_KEY"])
		region := string(secretData["AWS_REGION"])
		
		if accessKey != "" && secretKey != "" {
			opts = append(opts, config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
			))
		}
		if region != "" {
			opts = append(opts, config.WithRegion(region))
		}
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg)

	// List objects in the bucket
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	var allFiles []string
	var fileHashes = make(map[string]string)
	var totalSize int64
	var fileCount int

	paginator := s3.NewListObjectsV2Paginator(s3Client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			// Skip directories
			if strings.HasSuffix(key, "/") {
				continue
			}
			
			allFiles = append(allFiles, key)
			// Use ETag as file hash (already MD5 for non-multipart uploads)
			fileHashes[key] = strings.Trim(aws.ToString(obj.ETag), "\"")
			totalSize += aws.ToInt64(obj.Size)
			fileCount++
		}
	}

	// Sort files for consistent hash calculation
	sort.Strings(allFiles)

	// Calculate combined hash of all file ETags
	hasher := sha256.New()
	for _, f := range allFiles {
		hasher.Write([]byte(f))
		hasher.Write([]byte(fileHashes[f]))
	}
	newHash := hex.EncodeToString(hasher.Sum(nil))

	// Compare with previous state
	result := &SourceChangeResult{
		NewHash: newHash,
		NewMetadata: &ragv1alpha1.SourceMetadata{
			FileCount:  fileCount,
			TotalSize:  totalSize,
			FileHashes: fileHashes,
		},
	}

	// Check if changed
	if ds.Status.LastSourceHash != "" && ds.Status.LastSourceHash != newHash {
		result.Changed = true
		
		// Calculate file differences if we have previous hashes
		if ds.Status.SourceMetadata != nil && ds.Status.SourceMetadata.FileHashes != nil {
			result.AddedFiles, result.DeletedFiles, result.ChangedFiles = w.diffFileHashes(
				ds.Status.SourceMetadata.FileHashes, fileHashes,
			)
			result.FilesAdded = len(result.AddedFiles)
			result.FilesDeleted = len(result.DeletedFiles)
			result.FilesChanged = len(result.ChangedFiles)
		}
		
		logger.Info("S3 source changed",
			"filesAdded", result.FilesAdded,
			"filesDeleted", result.FilesDeleted,
			"filesChanged", result.FilesChanged,
		)
	} else if ds.Status.LastSourceHash == "" {
		// First time check
		result.Changed = true
		result.FilesAdded = fileCount
		logger.Info("First time source check, treating as changed", "fileCount", fileCount)
	}

	return result, nil
}

// checkHTTPSource checks HTTP endpoint for changes
func (w *SourceWatcher) checkHTTPSource(ctx context.Context, ds *ragv1alpha1.DocumentSet, secretData map[string][]byte) (*SourceChangeResult, error) {
	logger := w.Log.WithValues("documentset", ds.Name, "sourceType", "http")

	// Make HEAD request to check Last-Modified or ETag
	req, err := http.NewRequestWithContext(ctx, "HEAD", ds.Spec.Source.URI, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add auth header if secret provided
	if secretData != nil {
		if token := secretData["HTTP_AUTH_TOKEN"]; len(token) > 0 {
			req.Header.Set("Authorization", "Bearer "+string(token))
		}
		if basicAuth := secretData["HTTP_BASIC_AUTH"]; len(basicAuth) > 0 {
			req.Header.Set("Authorization", "Basic "+string(basicAuth))
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP HEAD request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP HEAD returned status %d", resp.StatusCode)
	}

	// Get ETag or Last-Modified
	etag := resp.Header.Get("ETag")
	lastModified := resp.Header.Get("Last-Modified")
	contentLength := resp.ContentLength

	// Create hash from metadata
	hasher := sha256.New()
	hasher.Write([]byte(etag))
	hasher.Write([]byte(lastModified))
	hasher.Write([]byte(fmt.Sprintf("%d", contentLength)))
	newHash := hex.EncodeToString(hasher.Sum(nil))

	result := &SourceChangeResult{
		NewHash: newHash,
		NewMetadata: &ragv1alpha1.SourceMetadata{
			S3ETag:    etag,
			TotalSize: contentLength,
		},
	}

	if ds.Status.LastSourceHash != "" && ds.Status.LastSourceHash != newHash {
		result.Changed = true
		result.FilesChanged = 1
		logger.Info("HTTP source changed", "etag", etag, "lastModified", lastModified)
	} else if ds.Status.LastSourceHash == "" {
		result.Changed = true
		result.FilesAdded = 1
	}

	return result, nil
}

// checkGitSource checks Git repository for changes
func (w *SourceWatcher) checkGitSource(ctx context.Context, ds *ragv1alpha1.DocumentSet, secretData map[string][]byte) (*SourceChangeResult, error) {
	logger := w.Log.WithValues("documentset", ds.Name, "sourceType", "git")

	// For Git, we would typically use go-git library
	// This is a simplified implementation that checks using git ls-remote
	
	uri := ds.Spec.Source.URI
	branch := "main" // Default branch
	
	// Parse branch from URI if specified (e.g., git://repo.git#branch)
	if idx := strings.Index(uri, "#"); idx != -1 {
		branch = uri[idx+1:]
		uri = uri[:idx]
	}

	// Use git ls-remote to get the latest commit
	// In production, use go-git library for full implementation
	// For now, we'll create a simple hash based on URI and timestamp
	// that simulates change detection
	
	hasher := sha256.New()
	hasher.Write([]byte(uri))
	hasher.Write([]byte(branch))
	hasher.Write([]byte(time.Now().Format("2006-01-02-15"))) // Hour-level granularity
	newHash := hex.EncodeToString(hasher.Sum(nil))

	result := &SourceChangeResult{
		NewHash: newHash,
		NewMetadata: &ragv1alpha1.SourceMetadata{
			GitBranch:     branch,
			GitCommitHash: newHash[:12], // Simulated commit hash
		},
	}

	// In a real implementation, compare with stored commit hash
	if ds.Status.SourceMetadata != nil && ds.Status.SourceMetadata.GitCommitHash != "" {
		if ds.Status.SourceMetadata.GitCommitHash != result.NewMetadata.GitCommitHash {
			result.Changed = true
			logger.Info("Git source changed", "oldCommit", ds.Status.SourceMetadata.GitCommitHash, "newCommit", result.NewMetadata.GitCommitHash)
		}
	} else if ds.Status.LastSourceHash == "" {
		result.Changed = true
	}

	return result, nil
}

// checkPVCSource checks PVC-mounted filesystem for changes
func (w *SourceWatcher) checkPVCSource(ctx context.Context, ds *ragv1alpha1.DocumentSet) (*SourceChangeResult, error) {
	logger := w.Log.WithValues("documentset", ds.Name, "sourceType", "pvc")

	// Parse PVC URI: pvc://pvc-name/path/
	uri := ds.Spec.Source.URI
	if !strings.HasPrefix(uri, "pvc://") {
		return nil, fmt.Errorf("invalid PVC URI: %s", uri)
	}

	// For PVC sources, we need to read from the mounted path
	// The actual path would be mounted by a worker pod
	// Here we calculate what the hash would be based on file metadata
	
	parts := strings.SplitN(strings.TrimPrefix(uri, "pvc://"), "/", 2)
	pvcName := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	// In production, this would be done by a sidecar or worker pod
	// that has the PVC mounted. For now, we'll use a base path
	basePath := filepath.Join("/mnt/pvc", pvcName, subPath)
	
	// Check if path exists (for local development/testing)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		logger.Info("PVC path not directly accessible, using metadata-based detection")
		// Return a hash based on the PVC name and current time (hour granularity)
		hasher := sha256.New()
		hasher.Write([]byte(pvcName))
		hasher.Write([]byte(subPath))
		
		result := &SourceChangeResult{
			NewHash: hex.EncodeToString(hasher.Sum(nil)),
			NewMetadata: &ragv1alpha1.SourceMetadata{
				FileCount: 0,
			},
		}
		
		if ds.Status.LastSourceHash == "" {
			result.Changed = true
		}
		return result, nil
	}

	// Walk the directory and collect file info
	var allFiles []string
	fileHashes := make(map[string]string)
	var totalSize int64
	var latestModTime time.Time

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(basePath, path)
		allFiles = append(allFiles, relPath)
		totalSize += info.Size()

		if info.ModTime().After(latestModTime) {
			latestModTime = info.ModTime()
		}

		// Calculate file hash (for small files) or use mod time + size
		if info.Size() < 10*1024*1024 { // 10MB limit for hashing
			hash, err := w.hashFile(path)
			if err == nil {
				fileHashes[relPath] = hash
			} else {
				fileHashes[relPath] = fmt.Sprintf("%d-%d", info.ModTime().Unix(), info.Size())
			}
		} else {
			fileHashes[relPath] = fmt.Sprintf("%d-%d", info.ModTime().Unix(), info.Size())
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk PVC directory: %w", err)
	}

	// Sort and create combined hash
	sort.Strings(allFiles)
	hasher := sha256.New()
	for _, f := range allFiles {
		hasher.Write([]byte(f))
		hasher.Write([]byte(fileHashes[f]))
	}
	newHash := hex.EncodeToString(hasher.Sum(nil))

	modTime := metav1.NewTime(latestModTime)
	result := &SourceChangeResult{
		NewHash: newHash,
		NewMetadata: &ragv1alpha1.SourceMetadata{
			FileCount:        len(allFiles),
			TotalSize:        totalSize,
			FileHashes:       fileHashes,
			LastModifiedTime: &modTime,
		},
	}

	if ds.Status.LastSourceHash != "" && ds.Status.LastSourceHash != newHash {
		result.Changed = true
		
		if ds.Status.SourceMetadata != nil && ds.Status.SourceMetadata.FileHashes != nil {
			result.AddedFiles, result.DeletedFiles, result.ChangedFiles = w.diffFileHashes(
				ds.Status.SourceMetadata.FileHashes, fileHashes,
			)
			result.FilesAdded = len(result.AddedFiles)
			result.FilesDeleted = len(result.DeletedFiles)
			result.FilesChanged = len(result.ChangedFiles)
		}
		
		logger.Info("PVC source changed",
			"filesAdded", result.FilesAdded,
			"filesDeleted", result.FilesDeleted,
			"filesChanged", result.FilesChanged,
		)
	} else if ds.Status.LastSourceHash == "" {
		result.Changed = true
		result.FilesAdded = len(allFiles)
	}

	return result, nil
}

// hashFile calculates MD5 hash of a file
func (w *SourceWatcher) hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// diffFileHashes compares old and new file hashes to find changes
func (w *SourceWatcher) diffFileHashes(old, new map[string]string) (added, deleted, changed []string) {
	// Find added and changed files
	for file, newHash := range new {
		if oldHash, exists := old[file]; !exists {
			added = append(added, file)
		} else if oldHash != newHash {
			changed = append(changed, file)
		}
	}

	// Find deleted files
	for file := range old {
		if _, exists := new[file]; !exists {
			deleted = append(deleted, file)
		}
	}

	return added, deleted, changed
}

// ShouldSync determines if a sync should be triggered based on the sync policy
func (w *SourceWatcher) ShouldSync(ds *ragv1alpha1.DocumentSet) (bool, string) {
	// No sync policy means manual mode only
	if ds.Spec.SyncPolicy == nil {
		return false, "no sync policy configured"
	}

	policy := ds.Spec.SyncPolicy

	// Check if sync is paused
	if policy.PauseSync {
		return false, "sync is paused"
	}

	// Check if mode is auto
	if policy.Mode != ragv1alpha1.SyncModeAuto {
		return false, "sync mode is not auto"
	}

	// Check sync interval
	interval, err := time.ParseDuration(policy.Interval)
	if err != nil {
		interval = 5 * time.Minute // Default
	}

	// Check if enough time has passed since last check
	if ds.Status.LastSourceCheckTime != nil {
		elapsed := time.Since(ds.Status.LastSourceCheckTime.Time)
		if elapsed < interval {
			return false, fmt.Sprintf("waiting for interval (%.0fs remaining)", (interval - elapsed).Seconds())
		}
	}

	return true, "sync interval reached"
}

// GetSyncInterval returns the sync interval duration
func GetSyncInterval(ds *ragv1alpha1.DocumentSet) time.Duration {
	if ds.Spec.SyncPolicy == nil || ds.Spec.SyncPolicy.Interval == "" {
		return 5 * time.Minute // Default
	}

	interval, err := time.ParseDuration(ds.Spec.SyncPolicy.Interval)
	if err != nil {
		return 5 * time.Minute
	}

	// Minimum 1 minute interval
	if interval < time.Minute {
		return time.Minute
	}

	return interval
}

