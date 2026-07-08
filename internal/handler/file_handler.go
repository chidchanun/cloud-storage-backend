package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"cloud-storage-backend/internal/middleware"
	"cloud-storage-backend/internal/models"
	"cloud-storage-backend/internal/repository"
	"cloud-storage-backend/internal/response"
)

const (
	multipartOverhead      = 1 << 20  // 1 MB
	defaultChunkUploadSize = 25 << 20 // 25 MB
	maxChunkUploadSize     = 90 << 20 // 90 MB
)

type chunkUploadSession struct {
	UserID       int    `json:"user_id"`
	OriginalName string `json:"original_name"`
	StoredName   string `json:"stored_name"`
	StoragePath  string `json:"storage_path"`
	SizeBytes    uint64 `json:"size_bytes"`
	FolderID     *int64 `json:"folder_id"`
	ChunkSize    int64  `json:"chunk_size"`
	TotalChunks  int    `json:"total_chunks"`
}

type FileHandler struct {
	fileRepo         *repository.FileRepository
	folderRepo       *repository.FolderRepository
	sharedFileRepo   *repository.SharedFileRepository
	sharedFolderRepo *repository.SharedFolderRepository
	userPlanRepo     *repository.UserPlanRepository
	uploadRoot       string
	maxUploadSize    int64
	chunkUploadSize  int64
}

func NewFileHandler(
	fileRepo *repository.FileRepository,
	folderRepo *repository.FolderRepository,
	sharedFileRepo *repository.SharedFileRepository,
	sharedFolderRepo *repository.SharedFolderRepository,
	userPlanRepo *repository.UserPlanRepository,
	uploadRoot string,
	maxUploadSize int64,
	chunkUploadSize int64,
) *FileHandler {
	if uploadRoot == "" {
		uploadRoot = "uploads"
	}

	if maxUploadSize <= 0 {
		maxUploadSize = 25 << 20
	}

	if chunkUploadSize <= 0 {
		chunkUploadSize = defaultChunkUploadSize
	}

	if chunkUploadSize > maxChunkUploadSize {
		chunkUploadSize = maxChunkUploadSize
	}

	return &FileHandler{
		fileRepo:         fileRepo,
		folderRepo:       folderRepo,
		sharedFileRepo:   sharedFileRepo,
		sharedFolderRepo: sharedFolderRepo,
		userPlanRepo:     userPlanRepo,
		uploadRoot:       uploadRoot,
		maxUploadSize:    maxUploadSize,
		chunkUploadSize:  chunkUploadSize,
	}
}

func (h *FileHandler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"โปรดเข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	var folderID *int64
	ownerUserID := claims.UserID

	folderIDParam := strings.TrimSpace(
		r.URL.Query().Get("folder_id"),
	)

	if folderIDParam != "" {
		parsedFolderID, err := strconv.ParseInt(
			folderIDParam,
			10,
			64,
		)
		if err != nil || parsedFolderID <= 0 {
			response.Error(
				w,
				http.StatusBadRequest,
				"รหัสโฟลเดอร์ไม่ถูกต้อง",
			)
			return
		}

		// ป้องกันการดูไฟล์ใน folder ของ user คนอื่น
		folder, err := h.findFolderForOwnerOrSharedViewer(
			r.Context(),
			parsedFolderID,
			claims.UserID,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.Error(
					w,
					http.StatusNotFound,
					"ไม่พบโฟลเดอร์",
				)
				return
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"ไม่สามารถโหลดโฟลเดอร์ได้",
			)
			return
		}

		folderID = &parsedFolderID
		ownerUserID = folder.UserID
	}

	files, err := h.fileRepo.FindAllByFolderID(
		r.Context(),
		ownerUserID,
		folderID,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดรายการไฟล์ได้",
		)
		return
	}

	fileResponses := make(
		[]models.UserFileResponse,
		0,
		len(files),
	)

	for i := range files {
		fileResponses = append(
			fileResponses,
			models.NewUserFileResponse(&files[i]),
		)
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"files": fileResponses,
			"total": len(fileResponses),
		},
	)
}

func (h *FileHandler) GetByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"please login before using this resource",
		)
		return
	}

	fileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid file id",
		)
		return
	}

	userFile, err := h.fileRepo.FindByIDAndUserID(
		r.Context(),
		fileID,
		claims.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหดลไฟล์จากเซิฟเวอร์ได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "file retrieved successfully",
			"file":    models.NewUserFileResponse(userFile),
		},
	)
}

// Rename เปลี่ยนชื่อที่แสดงของไฟล์
// PATCH /api/files/{id}/rename
func (h *FileHandler) Rename(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"โปรดเข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	// จำกัดขนาด JSON body
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var req models.RenameFileRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	// ต้อง Decode request body ก่อนตรวจค่า
	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
		return
	}

	if req.OriginalName == "" {
		response.Error(
			w,
			http.StatusBadRequest,
			"กรุณากรอกชื่อไฟล์",
		)
		return
	}

	// ตารางกำหนด original_name เป็น varchar(255)
	if len([]rune(req.OriginalName)) > 255 {
		response.Error(
			w,
			http.StatusBadRequest,
			"ชื่อไฟล์ต้องไม่เกิน 255 ตัวอักษร",
		)
		return
	}

	// ป้องกันการส่ง path มาแทนชื่อไฟล์
	// เช่น ../../secret.pdf หรือ folder/file.pdf
	cleanName := cleanOriginalName(req.OriginalName)
	if cleanName == "" || cleanName != req.OriginalName {
		response.Error(
			w,
			http.StatusBadRequest,
			"ชื่อไฟล์ไม่ถูกต้อง",
		)
		return
	}

	fileID, err := strconv.ParseInt(
		r.PathValue("id"),
		10,
		64,
	)
	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
		return
	}

	// ตรวจว่าไฟล์มีอยู่และเป็นของผู้ใช้คนนี้
	userFile, err := h.findFileForOwnerOrSharedEditor(
		r.Context(),
		fileID,
		claims.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์ที่ต้องการเปลี่ยนชื่อ",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดข้อมูลไฟล์ได้",
		)
		return
	}

	renamed, err := h.fileRepo.RenameFile(
		r.Context(),
		userFile.ID,
		userFile.UserID,
		cleanName,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถเปลี่ยนชื่อไฟล์ได้",
		)
		return
	}

	if !renamed {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบไฟล์ที่สามารถแก้ไขได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":       "เปลี่ยนชื่อไฟล์สำเร็จ",
			"file_id":       fileID,
			"original_name": cleanName,
		},
	)
}

// MoveFile ย้ายไฟล์ไปยังโฟลเดอร์อื่นหรือย้ายกลับ Root
// PATCH /api/files/{id}/move
func (h *FileHandler) MoveFile(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	fileID, err := strconv.ParseInt(
		r.PathValue("id"),
		10,
		64,
	)
	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
		return
	}

	// จำกัดขนาด JSON request body
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		64*1024,
	)

	var req models.MoveFileRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ข้อมูลที่ส่งมาไม่ถูกต้อง",
		)
		return
	}

	// ตรวจว่าไฟล์มีอยู่และเป็นของผู้ใช้คนนี้
	userFile, err := h.fileRepo.FindByIDAndUserID(
		r.Context(),
		fileID,
		claims.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์ที่ต้องการย้าย",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดข้อมูลไฟล์ได้",
		)
		return
	}

	// ถ้าจะย้ายเข้าโฟลเดอร์ ต้องตรวจว่าโฟลเดอร์มีอยู่จริง
	// เป็นของผู้ใช้ และยังไม่ถูก Soft Delete
	if req.FolderID != nil {
		if *req.FolderID <= 0 {
			response.Error(
				w,
				http.StatusBadRequest,
				"รหัสโฟลเดอร์ไม่ถูกต้อง",
			)
			return
		}

		_, err := h.folderRepo.FindByIDAndUserID(
			r.Context(),
			*req.FolderID,
			claims.UserID,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.Error(
					w,
					http.StatusNotFound,
					"ไม่พบโฟลเดอร์ปลายทาง",
				)
				return
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"ไม่สามารถตรวจสอบโฟลเดอร์ปลายทางได้",
			)
			return
		}
	}

	if err := h.fileRepo.MoveFile(
		r.Context(),
		userFile.ID,
		claims.UserID,
		req.FolderID,
	); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถย้ายไฟล์ได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message":   "ย้ายไฟล์สำเร็จ",
			"file_id":   fileID,
			"folder_id": req.FolderID,
		},
	)
}

// Download คือ handler สำหรับ GET /api/files/{id}/download
// ใช้ดาวน์โหลดไฟล์จริง โดยตรวจว่าไฟล์เป็นของผู้ใช้ที่ Login อยู่
func (h *FileHandler) Download(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"โปรดเข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	// อ่าน file ID จาก URL
	fileIDParam := r.PathValue("id")

	fileID, err := strconv.ParseInt(fileIDParam, 10, 64)

	if err != nil {
		response.Error(
			w,
			http.StatusBadRequest,
			"ไม่พบไฟล์",
		)
		return
	}

	// ค้นหาไฟล์พร้อมตรวจเจ้าของไฟล์
	userFile, err := h.findFileForOwnerOrSharedViewer(
		r.Context(),
		fileID,
		claims.UserID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหดลไฟล์จากเซิฟเวอร์ได้",
		)
		return
	}

	// ตรวจสอบว่า storage path อยู่ภายใน upload root จริง
	// ป้องกัน Path Traversal หรือ path ที่ผิดปกติ
	storagePath, err := h.resolveStoragePath(userFile.StoragePath)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่พบที่จัดเก็บไฟล์ในเซิฟเวอร์",
		)
		return
	}

	storedFile, err := os.Open(storagePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบที่เก็บไฟล์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดที่เก็บไฟล์ได้",
		)
		return
	}
	defer storedFile.Close()

	// อ่านข้อมูลไฟล์ เช่น ขนาดและเวลาแก้ไขล่าสุด
	fileInfo, err := storedFile.Stat()
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถอ่านข้อมูลไฟล์ในเซิฟเวอร์ได้",
		)
		return
	}

	// ป้องกันกรณี storage path ชี้ไปยัง directory
	if fileInfo.IsDir() {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบที่จัดเก็บไฟล์",
		)
	}

	contentType := userFile.MimeType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// สร้าง Content-Disposition อย่างปลอดภัย
	// Browser จะดาวน์โหลดโดยใช้ชื่อไฟล์เดิมของผู้ใช้
	contentDisposition := mime.FormatMediaType(
		"attachment",
		map[string]string{
			"filename": userFile.OriginalName,
		},
	)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition)

	// ป้องกัน Browser พยายามเดาชนิดไฟล์ใหม่
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// ไฟล์ของแต่ละ user ไม่ควรถูก cache แบบ public
	w.Header().Set("Cache-Control", "private, no-store")

	// ส่งไฟล์กลับไปยัง client
	// ServeContent รองรับ Content-Length และ Range Request ให้ด้วย
	http.ServeContent(
		w,
		r,
		userFile.OriginalName,
		fileInfo.ModTime(),
		storedFile,
	)
}

func (h *FileHandler) StartChunkUpload(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "please login before uploading files")
		return
	}

	var req models.StartChunkUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	originalName := cleanOriginalName(req.OriginalName)
	if originalName == "" || req.SizeBytes == 0 {
		response.Error(w, http.StatusBadRequest, "invalid file upload")
		return
	}

	currentPlan, err := h.userPlanRepo.FindCurrentByUserID(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(w, http.StatusForbidden, "กรุณาเลือกแพ็กเกจก่อนอัปโหลดไฟล์")
			return
		}

		response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบแพ็กเกจปัจจุบันได้")
		return
	}

	effectiveUploadLimit := h.effectiveUploadLimit(currentPlan.MaxFileSizeBytes)
	if req.SizeBytes > uint64(effectiveUploadLimit) {
		response.Error(w, http.StatusRequestEntityTooLarge, "ไฟล์มีขนาดใหญ่เกินขนาดสูงสุดของแพ็กเกจ")
		return
	}

	if currentPlan.MaxFiles != nil {
		fileCount, err := h.userPlanRepo.UserFileCount(r.Context(), claims.UserID)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบจำนวนไฟล์ได้")
			return
		}

		if fileCount >= uint64(*currentPlan.MaxFiles) {
			response.Error(w, http.StatusConflict, "จำนวนไฟล์เกินขีดจำกัดของแพ็กเกจ")
			return
		}
	}

	usedBytes, err := h.userPlanRepo.UsedStorageBytes(r.Context(), claims.UserID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบพื้นที่จัดเก็บได้")
		return
	}

	if req.SizeBytes > currentPlan.StorageLimitBytes ||
		usedBytes > currentPlan.StorageLimitBytes-req.SizeBytes {
		response.Error(w, http.StatusConflict, "พื้นที่จัดเก็บไม่พอสำหรับไฟล์นี้")
		return
	}

	if req.FolderID != nil {
		if *req.FolderID <= 0 {
			response.Error(w, http.StatusBadRequest, "รหัสโฟลเดอร์ไม่ถูกต้อง")
			return
		}

		_, err := h.folderRepo.FindByIDAndUserID(r.Context(), *req.FolderID, claims.UserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.Error(w, http.StatusNotFound, "ไม่พบโฟลเดอร์")
				return
			}

			response.Error(w, http.StatusInternalServerError, "ไม่สามารถตรวจสอบโฟลเดอร์ได้")
			return
		}
	}

	uploadID, err := generateChunkUploadID()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot create upload session")
		return
	}

	storedName, err := generateStoredName(originalName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot generate file name")
		return
	}

	chunkSize := h.chunkUploadSize
	if effectiveUploadLimit < chunkSize {
		chunkSize = effectiveUploadLimit
	}

	totalChunks := int((req.SizeBytes + uint64(chunkSize) - 1) / uint64(chunkSize))
	storagePath := filepath.Join(h.uploadRoot, strconv.Itoa(claims.UserID), storedName)

	session := chunkUploadSession{
		UserID:       claims.UserID,
		OriginalName: originalName,
		StoredName:   storedName,
		StoragePath:  storagePath,
		SizeBytes:    req.SizeBytes,
		FolderID:     req.FolderID,
		ChunkSize:    chunkSize,
		TotalChunks:  totalChunks,
	}

	if err := h.saveChunkUploadSession(uploadID, &session); err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot save upload session")
		return
	}

	response.JSON(w, http.StatusCreated, models.StartChunkUploadResponse{
		UploadID:  uploadID,
		ChunkSize: chunkSize,
	})
}

func (h *FileHandler) UploadChunk(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "please login before uploading files")
		return
	}

	uploadID := strings.TrimSpace(r.PathValue("upload_id"))
	if !isSafeChunkUploadID(uploadID) {
		response.Error(w, http.StatusBadRequest, "invalid upload session")
		return
	}

	chunkIndex, err := strconv.Atoi(strings.TrimSpace(r.PathValue("index")))
	if err != nil || chunkIndex < 0 {
		response.Error(w, http.StatusBadRequest, "invalid chunk index")
		return
	}

	session, err := h.loadChunkUploadSession(uploadID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "upload session not found")
		return
	}

	if session.UserID != claims.UserID || chunkIndex >= session.TotalChunks {
		response.Error(w, http.StatusBadRequest, "invalid upload chunk")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, session.ChunkSize+multipartOverhead)
	if err := r.ParseMultipartForm(session.ChunkSize); err != nil {
		response.Error(w, http.StatusRequestEntityTooLarge, "chunk is too large")
		return
	}

	chunkFile, fileHeader, err := r.FormFile("chunk")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "chunk is required")
		return
	}
	defer chunkFile.Close()

	if fileHeader.Size > session.ChunkSize {
		response.Error(w, http.StatusRequestEntityTooLarge, "chunk is too large")
		return
	}

	chunkDirectory := h.chunkUploadChunksDirectory(uploadID)
	if err := os.MkdirAll(chunkDirectory, 0750); err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot create chunk directory")
		return
	}

	chunkPath := filepath.Join(chunkDirectory, strconv.Itoa(chunkIndex)+".part")
	tempPath := chunkPath + ".tmp"

	destination, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0640)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot create chunk file")
		return
	}

	writtenBytes, copyErr := io.Copy(destination, io.LimitReader(chunkFile, session.ChunkSize+1))
	closeErr := destination.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tempPath)
		response.Error(w, http.StatusInternalServerError, "cannot store chunk")
		return
	}

	if writtenBytes > session.ChunkSize {
		_ = os.Remove(tempPath)
		response.Error(w, http.StatusRequestEntityTooLarge, "chunk is too large")
		return
	}

	if err := os.Rename(tempPath, chunkPath); err != nil {
		_ = os.Remove(tempPath)
		response.Error(w, http.StatusInternalServerError, "cannot finish chunk")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"message":     "chunk uploaded",
		"upload_id":   uploadID,
		"chunk_index": chunkIndex,
	})
}

func (h *FileHandler) CompleteChunkUpload(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		response.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "please login before uploading files")
		return
	}

	uploadID := strings.TrimSpace(r.PathValue("upload_id"))
	if !isSafeChunkUploadID(uploadID) {
		response.Error(w, http.StatusBadRequest, "invalid upload session")
		return
	}

	session, err := h.loadChunkUploadSession(uploadID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "upload session not found")
		return
	}

	if session.UserID != claims.UserID {
		response.Error(w, http.StatusForbidden, "upload session does not belong to this user")
		return
	}

	for index := 0; index < session.TotalChunks; index++ {
		if _, err := os.Stat(h.chunkUploadChunkPath(uploadID, index)); err != nil {
			response.Error(w, http.StatusBadRequest, "upload is not complete")
			return
		}
	}

	if err := os.MkdirAll(filepath.Dir(session.StoragePath), 0750); err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot create upload directory")
		return
	}

	destination, err := os.OpenFile(session.StoragePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0640)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "cannot create stored file")
		return
	}

	checksumWriter := sha256.New()
	writer := io.MultiWriter(destination, checksumWriter)
	var writtenBytes uint64
	var sniffBuffer []byte

	for index := 0; index < session.TotalChunks; index++ {
		chunkPath := h.chunkUploadChunkPath(uploadID, index)
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			_ = destination.Close()
			_ = os.Remove(session.StoragePath)
			response.Error(w, http.StatusInternalServerError, "cannot read chunk")
			return
		}

		if len(sniffBuffer) < 512 {
			neededBytes := 512 - len(sniffBuffer)
			buffer := make([]byte, neededBytes)
			readBytes, readErr := chunkFile.Read(buffer)
			if readErr != nil && !errors.Is(readErr, io.EOF) {
				_ = chunkFile.Close()
				_ = destination.Close()
				_ = os.Remove(session.StoragePath)
				response.Error(w, http.StatusInternalServerError, "cannot inspect uploaded file")
				return
			}

			sniffBuffer = append(sniffBuffer, buffer[:readBytes]...)
			if _, err := chunkFile.Seek(0, io.SeekStart); err != nil {
				_ = chunkFile.Close()
				_ = destination.Close()
				_ = os.Remove(session.StoragePath)
				response.Error(w, http.StatusInternalServerError, "cannot reset chunk")
				return
			}
		}

		copiedBytes, copyErr := io.Copy(writer, chunkFile)
		closeErr := chunkFile.Close()
		if copyErr != nil || closeErr != nil {
			_ = destination.Close()
			_ = os.Remove(session.StoragePath)
			response.Error(w, http.StatusInternalServerError, "cannot assemble uploaded file")
			return
		}

		writtenBytes += uint64(copiedBytes)
	}

	closeErr := destination.Close()
	if closeErr != nil {
		_ = os.Remove(session.StoragePath)
		response.Error(w, http.StatusInternalServerError, "cannot close stored file")
		return
	}

	if writtenBytes != session.SizeBytes {
		_ = os.Remove(session.StoragePath)
		response.Error(w, http.StatusBadRequest, "uploaded file size does not match")
		return
	}

	mimeType := http.DetectContentType(sniffBuffer)
	checksum := hex.EncodeToString(checksumWriter.Sum(nil))
	userFile := &models.UserFile{
		UserID:         claims.UserID,
		FolderID:       session.FolderID,
		OriginalName:   session.OriginalName,
		StoredName:     session.StoredName,
		StoragePath:    filepath.ToSlash(session.StoragePath),
		MimeType:       mimeType,
		SizeBytes:      writtenBytes,
		ChecksumSHA256: &checksum,
	}

	if err := h.fileRepo.Create(r.Context(), userFile); err != nil {
		_ = os.Remove(session.StoragePath)
		response.Error(w, http.StatusInternalServerError, "cannot save file metadata")
		return
	}

	_ = os.RemoveAll(h.chunkUploadSessionDirectory(uploadID))

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"message": "file uploaded successfully",
		"file":    models.NewUserFileResponse(userFile),
	})
}

func (h *FileHandler) Upload(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"method not allowed",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"please login before uploading files",
		)
		return
	}

	currentPlan, err := h.userPlanRepo.FindCurrentByUserID(
		r.Context(),
		claims.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusForbidden,
				"กรุณาเลือกแพ็กเกจก่อนอัปโหลดไฟล์",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถตรวจสอบแพ็กเกจปัจจุบันได้",
		)
		return
	}

	effectiveUploadLimit := h.effectiveUploadLimit(currentPlan.MaxFileSizeBytes)

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		effectiveUploadLimit+multipartOverhead,
	)

	if err := r.ParseMultipartForm(effectiveUploadLimit); err != nil {
		response.Error(
			w,
			http.StatusRequestEntityTooLarge,
			"file is too large",
		)
		return
	}

	var folderID *int64

	folderIDParam := strings.TrimSpace(
		r.FormValue("folder_id"),
	)

	if folderIDParam != "" {
		parsedFolderID, err := strconv.ParseInt(
			folderIDParam,
			10,
			64,
		)
		if err != nil || parsedFolderID <= 0 {
			response.Error(
				w,
				http.StatusBadRequest,
				"รหัสโฟลเดอร์ไม่ถูกต้อง",
			)
			return
		}

		// ตรวจว่าโฟลเดอร์เป็นของผู้ใช้จริง
		_, err = h.folderRepo.FindByIDAndUserID(
			r.Context(),
			parsedFolderID,
			claims.UserID,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				response.Error(
					w,
					http.StatusNotFound,
					"ไม่พบโฟลเดอร์",
				)
				return
			}

			response.Error(
				w,
				http.StatusInternalServerError,
				"ไม่สามารถตรวจสอบโฟลเดอร์ได้",
			)
			return
		}

		folderID = &parsedFolderID
	}

	uploadFile, fileHeader, err := r.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			response.Error(
				w,
				http.StatusBadRequest,
				"please provide a file",
			)
			return
		}

		response.Error(
			w,
			http.StatusBadRequest,
			"cannot read uploaded file",
		)
		return
	}
	defer uploadFile.Close()

	originalName := cleanOriginalName(fileHeader.Filename)
	if originalName == "" {
		response.Error(
			w,
			http.StatusBadRequest,
			"invalid file name",
		)
		return
	}

	if fileHeader.Size > effectiveUploadLimit {
		response.Error(
			w,
			http.StatusRequestEntityTooLarge,
			"ไฟล์มีขนาดใหญ่เกินขนาดสูงสุดของแพ็กเกจ",
		)
		return
	}

	if currentPlan.MaxFiles != nil {
		fileCount, err := h.userPlanRepo.UserFileCount(
			r.Context(),
			claims.UserID,
		)
		if err != nil {
			response.Error(
				w,
				http.StatusInternalServerError,
				"ไม่สามารถตรวจสอบจำนวนไฟล์ได้",
			)
			return
		}

		if fileCount >= uint64(*currentPlan.MaxFiles) {
			response.Error(
				w,
				http.StatusConflict,
				"จำนวนไฟล์เกินขีดจำกัดของแพ็กเกจ",
			)
			return
		}
	}

	usedBytes, err := h.userPlanRepo.UsedStorageBytes(
		r.Context(),
		claims.UserID,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถตรวจสอบพื้นที่จัดเก็บได้",
		)
		return
	}

	uploadSizeBytes := uint64(fileHeader.Size)
	if uploadSizeBytes > currentPlan.StorageLimitBytes ||
		usedBytes > currentPlan.StorageLimitBytes-uploadSizeBytes {
		response.Error(
			w,
			http.StatusConflict,
			"พื้นที่จัดเก็บไม่พอสำหรับไฟล์นี้",
		)
		return
	}

	storedName, err := generateStoredName(originalName)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot generate file name",
		)
		return
	}

	userDirectory := filepath.Join(
		h.uploadRoot,
		strconv.Itoa(claims.UserID),
	)

	if err := os.MkdirAll(userDirectory, 0750); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot create upload directory",
		)
		return
	}

	storagePath := filepath.Join(
		userDirectory,
		storedName,
	)

	headerBuffer := make([]byte, 512)
	bytesRead, err := uploadFile.Read(headerBuffer)
	if err != nil && !errors.Is(err, io.EOF) {
		response.Error(
			w,
			http.StatusBadRequest,
			"cannot inspect uploaded file",
		)
		return
	}

	mimeType := http.DetectContentType(headerBuffer[:bytesRead])

	if _, err := uploadFile.Seek(0, io.SeekStart); err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot reset uploaded file",
		)
		return
	}

	destination, err := os.OpenFile(
		storagePath,
		os.O_CREATE|os.O_WRONLY|os.O_EXCL,
		0640,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot create stored file",
		)
		return
	}

	checksumWriter := sha256.New()
	writer := io.MultiWriter(
		destination,
		checksumWriter,
	)

	writtenBytes, copyErr := io.Copy(
		writer,
		io.LimitReader(uploadFile, effectiveUploadLimit+1),
	)

	closeErr := destination.Close()

	if copyErr != nil {
		_ = os.Remove(storagePath)

		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot store uploaded file",
		)
		return
	}

	if closeErr != nil {
		_ = os.Remove(storagePath)

		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot close stored file",
		)
		return
	}

	if writtenBytes > effectiveUploadLimit {
		_ = os.Remove(storagePath)

		response.Error(
			w,
			http.StatusRequestEntityTooLarge,
			"file exceeds maximum upload size",
		)
		return
	}

	checksum := hex.EncodeToString(checksumWriter.Sum(nil))
	userFile := &models.UserFile{
		UserID:         claims.UserID,
		FolderID:       folderID,
		OriginalName:   originalName,
		StoredName:     storedName,
		StoragePath:    filepath.ToSlash(storagePath),
		MimeType:       mimeType,
		SizeBytes:      uint64(writtenBytes),
		ChecksumSHA256: &checksum,
	}

	if err := h.fileRepo.Create(r.Context(), userFile); err != nil {
		_ = os.Remove(storagePath)

		response.Error(
			w,
			http.StatusInternalServerError,
			"cannot save file metadata",
		)
		return
	}

	response.JSON(
		w,
		http.StatusCreated,
		map[string]interface{}{
			"message": "file uploaded successfully",
			"file":    models.NewUserFileResponse(userFile),
		},
	)
}

func (h *FileHandler) effectiveUploadLimit(planMaxFileSize uint64) int64 {
	limit := h.maxUploadSize
	if limit <= 0 {
		limit = 25 << 20
	}

	if planMaxFileSize == 0 {
		return limit
	}

	if planMaxFileSize < uint64(limit) {
		return int64(planMaxFileSize)
	}

	return limit
}

func generateChunkUploadID() (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(randomBytes), nil
}

func isSafeChunkUploadID(uploadID string) bool {
	if len(uploadID) != 32 {
		return false
	}

	for _, char := range uploadID {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}

	return true
}

func (h *FileHandler) chunkUploadRootDirectory() string {
	return filepath.Join(h.uploadRoot, ".chunks")
}

func (h *FileHandler) chunkUploadSessionDirectory(uploadID string) string {
	return filepath.Join(h.chunkUploadRootDirectory(), uploadID)
}

func (h *FileHandler) chunkUploadSessionPath(uploadID string) string {
	return filepath.Join(h.chunkUploadSessionDirectory(uploadID), "session.json")
}

func (h *FileHandler) chunkUploadChunksDirectory(uploadID string) string {
	return filepath.Join(h.chunkUploadSessionDirectory(uploadID), "chunks")
}

func (h *FileHandler) chunkUploadChunkPath(uploadID string, chunkIndex int) string {
	return filepath.Join(
		h.chunkUploadChunksDirectory(uploadID),
		strconv.Itoa(chunkIndex)+".part",
	)
}

func (h *FileHandler) saveChunkUploadSession(
	uploadID string,
	session *chunkUploadSession,
) error {
	sessionDirectory := h.chunkUploadSessionDirectory(uploadID)
	if err := os.MkdirAll(sessionDirectory, 0750); err != nil {
		return err
	}

	sessionFile, err := os.OpenFile(
		h.chunkUploadSessionPath(uploadID),
		os.O_CREATE|os.O_WRONLY|os.O_EXCL,
		0640,
	)
	if err != nil {
		return err
	}
	defer sessionFile.Close()

	return json.NewEncoder(sessionFile).Encode(session)
}

func (h *FileHandler) loadChunkUploadSession(uploadID string) (*chunkUploadSession, error) {
	sessionFile, err := os.Open(h.chunkUploadSessionPath(uploadID))
	if err != nil {
		return nil, err
	}
	defer sessionFile.Close()

	var session chunkUploadSession
	if err := json.NewDecoder(sessionFile).Decode(&session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (h *FileHandler) SoftDelete(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"โปรดเข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	// อ่าน file ID จาก URL
	fileIDParam := r.PathValue("id")

	fileID, err := strconv.ParseInt(fileIDParam, 10, 64)
	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"ไม่พบไฟล์",
		)
		return
	}

	// ดึงข้อมูลไฟล์ก่อนลบ
	// ใช้ทั้ง file ID และ user ID เพื่อป้องกันการลบไฟล์ของคนอื่น
	userFile, err := h.fileRepo.FindByIDAndUserID(
		r.Context(),
		fileID,
		claims.UserID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusBadRequest,
				"ไม่พบไฟล์ที่ต้องการลบ",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดข้อมูลไฟล์จากเซิฟเวอร์ได้",
		)
		return
	}

	// ทำ Soft Delete ในฐานข้อมูลก่อน
	deleted, err := h.fileRepo.SoftDelete(
		r.Context(),
		userFile.ID,
		claims.UserID,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถลบได้ โปรดลองอีกครั้ง",
		)
		return
	}

	if !deleted {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบไฟล์",
		)
		return
	}

	// ตรวจสอบว่า path อยู่ภายใน upload root
	// storagePath, err := h.resolveStoragePath(
	// 	userFile.StoragePath,
	// )

	// if err != nil {
	// 	response.Error(
	// 		w,
	// 		http.StatusInternalServerError,
	// 		"ไม่พบตำแหน่งไฟล์",
	// 	)

	// 	return
	// }

	// // ลบไฟล์จริงจาก Server
	// // ถ้าไฟล์ไม่มีอยู่แล้ว ถือว่า metadata ถูกลบสำเร็จ

	// if err := os.Remove(storagePath); err != nil &&
	// 	!errors.Is(err, os.ErrNotExist) {
	// 	// ไม่ควรส่ง path ภายใน Server กลับไปยัง Client
	// 	// จึงบันทึกรายละเอียดไว้เฉพาะใน Server log
	// 	log.Printf(
	// 		"warning: cannot remove stored file %q: %v",
	// 		storagePath,
	// 		err,
	// 	)
	// }

	// // ลองลบโฟลเดอร์ของ user ถ้าโฟลเดอร์ว่าง
	// // os.Remove จะไม่ลบหากยังมีไฟล์อื่นอยู่
	// _ = os.Remove(filepath.Dir(storagePath))

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "file deleted successfully",
			"file_id": fileID,
		},
	)
}

// Detele From Server And Delete Row DB
func (h *FileHandler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodDelete {
		response.Error(
			w,
			http.StatusMethodNotAllowed,
			"ไม่อนุญาตให้ใช้ Method อื่น",
		)
		return
	}

	claims, ok := middleware.UserFromContext(r.Context())

	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	fileID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
		return
	}

	// ตรวจสอบความเป็นเจ้าของไฟล์
	UserFile, err := h.fileRepo.FindTrashByIDAndUserID(
		r.Context(),
		fileID,
		claims.UserID,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์ที่ต้องการลบ",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดข้อมูลไฟล์จากเซิฟเวอร์ได้",
		)
		return
	}

	// ลบ row ใน db
	deleted, err := h.fileRepo.Delete(
		r.Context(),
		UserFile.ID,
		UserFile.UserID,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถลบข้อมูลไฟล์ได้",
		)

		return
	}

	if !deleted {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบไฟล์",
		)

		return
	}

	// ลบจากไฟล์ใน Server
	// ตรวจสอบว่า path อยู่ภายใน upload root
	storagePath, err := h.resolveStoragePath(
		UserFile.StoragePath,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่พบตำแหน่งไฟล์",
		)

		return
	}

	// ลบไฟล์จริงจาก Server
	// ถ้าไฟล์ไม่มีอยู่แล้ว ถือว่า metadata ถูกลบสำเร็จ

	if err := os.Remove(storagePath); err != nil &&
		!errors.Is(err, os.ErrNotExist) {
		// ไม่ควรส่ง path ภายใน Server กลับไปยัง Client
		// จึงบันทึกรายละเอียดไว้เฉพาะใน Server log
		log.Printf(
			"warning: cannot remove stored file %q: %v",
			storagePath,
			err,
		)
	}

	// ลองลบโฟลเดอร์ของ user ถ้าโฟลเดอร์ว่าง
	// os.Remove จะไม่ลบหากยังมีไฟล์อื่นอยู่
	_ = os.Remove(filepath.Dir(storagePath))

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "file deleted successfully",
			"file_id": fileID,
		},
	)
}

// Restore คือ handler สำหรับ PATCH /api/files/{id}/restore
func (h *FileHandler) Restore(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"โปรดเข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	fileID, err := strconv.ParseInt(
		r.PathValue("id"),
		10,
		64,
	)

	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
	}

	restored, err := h.fileRepo.Restore(
		r.Context(),
		fileID,
		claims.UserID,
	)

	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถกู้คืนไฟล์ได้",
		)
		return
	}

	if !restored {
		response.Error(
			w,
			http.StatusNotFound,
			"ไม่พบไฟล์ที่สามารถกู้คืนได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "กู้คืนไฟล์สำเร็จ",
			"file_id": fileID,
		},
	)
}

// SearchByFileName ค้นหาไฟล์จากชื่อไฟล์ของผู้ใช้ที่เข้าสู่ระบบ
//
// GET /api/files/search?q=report
func (h *FileHandler) SearchByFileName(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"เข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	keyword := strings.TrimSpace(
		r.URL.Query().Get("q"),
	)

	if keyword == "" {
		response.Error(
			w,
			http.StatusBadRequest,
			"กรุณาระบุชื่อไฟล์ที่ต้องการค้นหา",
		)
		return
	}

	// ป้องกันคำค้นยาวเกินความจำเป็น
	if len([]rune(keyword)) > 255 {
		response.Error(
			w,
			http.StatusBadRequest,
			"คำค้นหาต้องไม่เกิน 255 ตัวอักษร",
		)
		return
	}

	files, err := h.fileRepo.SearchByFileName(
		r.Context(),
		claims.UserID,
		keyword,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถค้นหาไฟล์ได้",
		)
		return
	}

	fileResponses := make(
		[]models.UserFileResponse,
		0,
		len(files),
	)

	for i := range files {
		fileResponses = append(
			fileResponses,
			models.NewUserFileResponse(&files[i]),
		)
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "ค้นหาไฟล์สำเร็จ",
			"keyword": keyword,
			"files":   fileResponses,
			"total":   len(fileResponses),
		},
	)
}

// ListTrash ดึงรายการไฟล์ทั้งหมดในถังขยะของผู้ใช้
func (h *FileHandler) ListTrash(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"โปรดเข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	files, err := h.fileRepo.FindAllTrashByUser(
		r.Context(),
		claims.UserID,
	)
	if err != nil {
		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดไฟล์ในถังขยะได้",
		)
		return
	}

	fileResponses := make(
		[]models.UserFileResponse,
		0,
		len(files),
	)

	for i := range files {
		fileResponses = append(
			fileResponses,
			models.NewUserFileResponse(&files[i]),
		)
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "โหลดรายการไฟล์ในถังขยะสำเร็จ",
			"files":   fileResponses,
			"total":   len(fileResponses),
		},
	)
}

// GetTrashByID ดึงข้อมูลไฟล์ในถังขยะตาม ID
func (h *FileHandler) GetTrashByID(
	w http.ResponseWriter,
	r *http.Request,
) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		response.Error(
			w,
			http.StatusUnauthorized,
			"โปรดเข้าสู่ระบบก่อนใช้งาน",
		)
		return
	}

	fileID, err := strconv.ParseInt(
		r.PathValue("id"),
		10,
		64,
	)
	if err != nil || fileID <= 0 {
		response.Error(
			w,
			http.StatusBadRequest,
			"รหัสไฟล์ไม่ถูกต้อง",
		)
		return
	}

	// ต้องใช้ method สำหรับค้นหาไฟล์ที่ deleted_at IS NOT NULL
	userFile, err := h.fileRepo.FindTrashByIDAndUserID(
		r.Context(),
		fileID,
		claims.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.Error(
				w,
				http.StatusNotFound,
				"ไม่พบไฟล์ในถังขยะ",
			)
			return
		}

		response.Error(
			w,
			http.StatusInternalServerError,
			"ไม่สามารถโหลดข้อมูลไฟล์จากเซิร์ฟเวอร์ได้",
		)
		return
	}

	response.JSON(
		w,
		http.StatusOK,
		map[string]interface{}{
			"message": "โหลดข้อมูลไฟล์ในถังขยะสำเร็จ",
			"file":    models.NewUserFileResponse(userFile),
		},
	)
}

func (h *FileHandler) findFileForOwnerOrSharedEditor(
	ctx context.Context,
	fileID int64,
	userID int,
) (*models.UserFile, error) {
	userFile, err := h.fileRepo.FindByIDAndUserID(
		ctx,
		fileID,
		userID,
	)
	if err == nil {
		return userFile, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	permission, err := h.sharedFileRepo.FindPermissionByFileIDAndUserID(
		ctx,
		fileID,
		userID,
	)
	if err != nil {
		return nil, err
	}

	if permission != "editor" {
		return nil, sql.ErrNoRows
	}

	return h.fileRepo.FindByID(
		ctx,
		fileID,
	)
}

func (h *FileHandler) findFileForOwnerOrSharedViewer(
	ctx context.Context,
	fileID int64,
	userID int,
) (*models.UserFile, error) {
	userFile, err := h.fileRepo.FindByIDAndUserID(
		ctx,
		fileID,
		userID,
	)
	if err == nil {
		return userFile, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	permission, err := h.sharedFileRepo.FindPermissionByFileIDAndUserID(
		ctx,
		fileID,
		userID,
	)
	if err == nil {
		if permission == models.SharedPermissionViewer || permission == models.SharedPermissionEditor {
			return h.fileRepo.FindByID(ctx, fileID)
		}

		return nil, sql.ErrNoRows
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if h.sharedFolderRepo == nil {
		return nil, sql.ErrNoRows
	}

	userFile, err = h.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}

	if userFile.FolderID == nil {
		return nil, sql.ErrNoRows
	}

	permission, err = h.sharedFolderRepo.FindPermissionByFolderTreeAndUserID(
		ctx,
		*userFile.FolderID,
		userID,
	)
	if err != nil {
		return nil, err
	}

	if permission != models.SharedPermissionViewer && permission != models.SharedPermissionEditor {
		return nil, sql.ErrNoRows
	}

	return userFile, nil
}

func (h *FileHandler) findFolderForOwnerOrSharedViewer(
	ctx context.Context,
	folderID int64,
	userID int,
) (*models.UserFolder, error) {
	folder, err := h.folderRepo.FindByIDAndUserID(
		ctx,
		folderID,
		userID,
	)
	if err == nil {
		return folder, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if h.sharedFolderRepo == nil {
		return nil, sql.ErrNoRows
	}

	permission, err := h.sharedFolderRepo.FindPermissionByFolderTreeAndUserID(
		ctx,
		folderID,
		userID,
	)
	if err != nil {
		return nil, err
	}

	if permission != models.SharedPermissionViewer && permission != models.SharedPermissionEditor {
		return nil, sql.ErrNoRows
	}

	return h.folderRepo.FindByID(ctx, folderID)
}

func cleanOriginalName(fileName string) string {
	fileName = strings.TrimSpace(fileName)
	fileName = strings.ReplaceAll(fileName, `\`, `/`)
	fileName = path.Base(fileName)

	if fileName == "." || fileName == "/" {
		return ""
	}

	return fileName
}

func generateStoredName(originalName string) (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	extension := strings.ToLower(filepath.Ext(originalName))
	if len(extension) > 20 {
		extension = ""
	}

	return hex.EncodeToString(randomBytes) + extension, nil
}

// resolveStoragePath ตรวจสอบว่าไฟล์อยู่ภายใน uploadRoot จริง
// ป้องกัน path เช่น ../../secret.txt
func (h *FileHandler) resolveStoragePath(
	storagePath string,
) (string, error) {
	// Path ในฐานข้อมูลเก็บด้วย slash แบบกลาง
	// แปลงกลับเป็น separator ของ OS เช่น \ สำหรับ Windows
	cleanStoragePath := filepath.Clean(
		filepath.FromSlash(storagePath),
	)

	uploadRootAbsolute, err := filepath.Abs(h.uploadRoot)
	if err != nil {
		return "", err
	}

	fileAbsolute, err := filepath.Abs(cleanStoragePath)
	if err != nil {
		return "", err
	}

	// หาตำแหน่งไฟล์เมื่อเทียบกับ upload root
	relativePath, err := filepath.Rel(
		uploadRootAbsolute,
		fileAbsolute,
	)
	if err != nil {
		return "", err
	}

	// ถ้าขึ้นต้นด้วย .. แปลว่าไฟล์อยู่นอก upload root
	if relativePath == ".." ||
		strings.HasPrefix(
			relativePath,
			".."+string(os.PathSeparator),
		) ||
		filepath.IsAbs(relativePath) {
		return "", errors.New(
			"storage path is outside upload root",
		)
	}

	return fileAbsolute, nil
}
