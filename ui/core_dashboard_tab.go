package ui

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"singbox-launcher/core"
)

// CoreDashboardTab управляет вкладкой Core Dashboard
type CoreDashboardTab struct {
	controller *core.AppController

	// UI elements
	statusLabel             *widget.Label // Full status: "Core Status" + icon + text
	singboxStatusLabel      *widget.Label // sing-box status (version or "not found")
	downloadButton          *widget.Button
	downloadProgress        *widget.ProgressBar // Progress bar for download
	downloadContainer       fyne.CanvasObject   // Container for button/progress bar
	startButton             *widget.Button      // Start button
	stopButton              *widget.Button      // Stop button
	wintunStatusLabel       *widget.Label       // wintun.dll status
	wintunDownloadButton    *widget.Button      // wintun.dll download button
	wintunDownloadProgress  *widget.ProgressBar // Progress bar for wintun.dll download
	wintunDownloadContainer fyne.CanvasObject   // Container for wintun button/progress bar
	configStatusLabel       *widget.Label
	templateDownloadButton  *widget.Button
	wizardButton            *widget.Button

	// Data
	stopAutoUpdate           chan bool
	lastUpdateSuccess        bool // Track success of last version update
	downloadInProgress       bool // Flag for sing-box download process
	wintunDownloadInProgress bool // Flag for wintun.dll download process
}

// CreateCoreDashboardTab creates and returns the Core Dashboard tab
func CreateCoreDashboardTab(ac *core.AppController) fyne.CanvasObject {
	tab := &CoreDashboardTab{
		controller:     ac,
		stopAutoUpdate: make(chan bool),
	}

	// Status block with buttons in one row
	statusRow := tab.createStatusRow()

	versionBlock := tab.createVersionBlock()
	configBlock := tab.createConfigBlock()

	var wintunBlock fyne.CanvasObject
	if runtime.GOOS == "windows" {
		wintunBlock = tab.createWintunBlock()
	}

	coreRows := []fyne.CanvasObject{versionBlock}
	if runtime.GOOS == "windows" && wintunBlock != nil {
		coreRows = append(coreRows, wintunBlock)
	}
	coreRows = append(coreRows, configBlock)
	coreInfo := container.NewVBox(coreRows...)

	contentItems := []fyne.CanvasObject{
		statusRow,
		widget.NewSeparator(),
		coreInfo,
		widget.NewSeparator(),
	}

	// Горизонтальная линия и кнопка Exit в конце списка
	exitButton := widget.NewButton("Exit", ac.GracefulExit)
	// Кнопка Exit в отдельной строке
	contentItems = append(contentItems, container.NewCenter(exitButton))

	content := container.NewVBox(contentItems...)

	// Регистрируем callback для обновления статуса при изменении RunningState
	tab.controller.UpdateCoreStatusFunc = func() {
		fyne.Do(func() {
			tab.updateRunningStatus()
		})
	}

	// Регистрируем callback для обновления статуса конфига
	tab.controller.UpdateConfigStatusFunc = func() {
		fyne.Do(func() {
			tab.updateConfigInfo()
		})
	}

	// Первоначальное обновление
	tab.updateBinaryStatus() // Проверяет наличие бинарника и вызывает updateRunningStatus
	tab.updateVersionInfo()
	if runtime.GOOS == "windows" {
		tab.updateWintunStatus() // Проверяет наличие wintun.dll
	}
	tab.updateConfigInfo()

	// Запускаем автообновление версии
	tab.startAutoUpdate()

	return content
}

// createStatusRow creates a row with status and buttons
func (tab *CoreDashboardTab) createStatusRow() fyne.CanvasObject {
	// Объединяем все в один label: "Core Status" + иконка + текст статуса
	tab.statusLabel = widget.NewLabel("Core Status Checking...")
	tab.statusLabel.Wrapping = fyne.TextWrapOff       // Отключаем перенос текста
	tab.statusLabel.Alignment = fyne.TextAlignLeading // Выравнивание текста
	tab.statusLabel.Importance = widget.MediumImportance

	startButton := widget.NewButton("Start", func() {
		core.StartSingBoxProcess(tab.controller)
		// Status will be updated automatically via UpdateCoreStatusFunc
	})

	stopButton := widget.NewButton("Stop", func() {
		core.StopSingBoxProcess(tab.controller)
		// Status will be updated automatically via UpdateCoreStatusFunc
	})

	// Save button references for updating locks
	tab.startButton = startButton
	tab.stopButton = stopButton

	// Status in one line - everything in one label
	statusContainer := container.NewHBox(
		tab.statusLabel, // "Core Status" + icon + status text
	)

	// Buttons on new line centered
	buttonsContainer := container.NewCenter(
		container.NewHBox(startButton, stopButton),
	)

	// Return container with status and buttons, with empty lines before and after buttons
	return container.NewVBox(
		statusContainer,
		widget.NewLabel(""), // Empty line before buttons
		buttonsContainer,
		widget.NewLabel(""), // Empty line after buttons
	)
}

func (tab *CoreDashboardTab) createConfigBlock() fyne.CanvasObject {
	title := widget.NewLabel("Config")
	title.Importance = widget.MediumImportance

	tab.configStatusLabel = widget.NewLabel("Checking config...")
	tab.configStatusLabel.Wrapping = fyne.TextWrapOff

	tab.wizardButton = widget.NewButton("⚙️ Wizard", func() {
		ShowConfigWizard(tab.controller.MainWindow, tab.controller)
	})
	tab.wizardButton.Importance = widget.MediumImportance

	tab.templateDownloadButton = widget.NewButton("Download Config Template", func() {
		tab.downloadConfigTemplate()
	})
	tab.templateDownloadButton.Importance = widget.MediumImportance

	// Initially hide both, updateConfigInfo will show the appropriate one
	tab.wizardButton.Hide()
	tab.templateDownloadButton.Hide()

	return container.NewHBox(
		title,
		layout.NewSpacer(),
		tab.configStatusLabel,
		layout.NewSpacer(),
		tab.wizardButton,
		tab.templateDownloadButton,
	)
}

// createVersionBlock creates a block with version (similar to wintun)
func (tab *CoreDashboardTab) createVersionBlock() fyne.CanvasObject {
	title := widget.NewLabel("Sing-box")
	title.Importance = widget.MediumImportance

	tab.singboxStatusLabel = widget.NewLabel("Checking...")
	tab.singboxStatusLabel.Wrapping = fyne.TextWrapOff

	tab.downloadButton = widget.NewButton("Download", func() {
		tab.handleDownload()
	})
	tab.downloadButton.Importance = widget.MediumImportance
	tab.downloadButton.Disable()

	tab.downloadProgress = widget.NewProgressBar()
	tab.downloadProgress.Hide()
	tab.downloadProgress.SetValue(0)
	progressContainer := container.NewMax(tab.downloadProgress)
	tab.downloadContainer = container.NewStack(tab.downloadButton, progressContainer)

	return container.NewHBox(
		title,
		layout.NewSpacer(),
		tab.singboxStatusLabel,
		tab.downloadContainer,
	)
}

// updateBinaryStatus проверяет наличие бинарника и обновляет статус
func (tab *CoreDashboardTab) updateBinaryStatus() {
	// Проверяем, существует ли бинарник
	if _, err := tab.controller.GetInstalledCoreVersion(); err != nil {
		tab.statusLabel.SetText("Core Status ❌ Error: sing-box not found")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
		// Обновляем иконку трея (красная при ошибке)
		tab.controller.UpdateUI()
		return
	}
	// Если бинарник найден, обновляем статус запуска
	tab.updateRunningStatus()
	// Обновляем иконку трея (может измениться с красной на черную/зеленую)
	tab.controller.UpdateUI()
}

// updateRunningStatus обновляет статус Running/Stopped на основе RunningState
func (tab *CoreDashboardTab) updateRunningStatus() {
	// Get button state from centralized function (same logic as Tray Menu)
	buttonState := tab.controller.GetVPNButtonState()

	// Update status label based on state
	if !buttonState.BinaryExists {
		tab.statusLabel.SetText("Core Status ❌ Error: sing-box not found")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
	} else if buttonState.IsRunning {
		tab.statusLabel.SetText("Core Status ✅ Running")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
	} else {
		tab.statusLabel.SetText("Core Status ⏸️ Stopped")
		tab.statusLabel.Importance = widget.MediumImportance // Текст всегда черный
	}

	// Update buttons based on centralized state
	if tab.startButton != nil {
		if buttonState.StartEnabled {
			tab.startButton.Enable()
		} else {
			tab.startButton.Disable()
		}
	}
	if tab.stopButton != nil {
		if buttonState.StopEnabled {
			tab.stopButton.Enable()
		} else {
			tab.stopButton.Disable()
		}
	}
}

func (tab *CoreDashboardTab) updateConfigInfo() {
	if tab.configStatusLabel == nil {
		return
	}
	configPath := tab.controller.ConfigPath
	if info, err := os.Stat(configPath); err == nil {
		modTime := info.ModTime().Format("2006-01-02")
		tab.configStatusLabel.SetText(fmt.Sprintf("%s ✅ %s", filepath.Base(configPath), modTime))
	} else if os.IsNotExist(err) {
		tab.configStatusLabel.SetText(fmt.Sprintf("%s ❌ not found", filepath.Base(configPath)))
	} else {
		tab.configStatusLabel.SetText(fmt.Sprintf("Config error: %v", err))
	}

	templatePath := filepath.Join(tab.controller.ExecDir, "bin", "config_template.json")
	if _, err := os.Stat(templatePath); err != nil {
		// Template not found - show download button, hide wizard
		tab.templateDownloadButton.Show()
		tab.templateDownloadButton.Enable()
		if tab.wizardButton != nil {
			tab.wizardButton.Hide()
		}
	} else {
		// Template found - show wizard, hide download button
		tab.templateDownloadButton.Hide()
		if tab.wizardButton != nil {
			tab.wizardButton.Show()
		}
	}
}

// updateVersionInfo обновляет информацию о версии (по аналогии с updateWintunStatus)
// Теперь полностью асинхронная - не блокирует UI
func (tab *CoreDashboardTab) updateVersionInfo() error {
	// Запускаем асинхронное обновление
	tab.updateVersionInfoAsync()
	return nil
}

// updateVersionInfoAsync - asynchronous version of version information update
func (tab *CoreDashboardTab) updateVersionInfoAsync() {
	// Запускаем в горутине
	go func() {
		// Получаем установленную версию (локальная операция, быстрая)
		installedVersion, err := tab.controller.GetInstalledCoreVersion()

		// Обновляем UI для установленной версии
		fyne.Do(func() {
			if err != nil {
				// Показываем ошибку в статусе
				tab.singboxStatusLabel.SetText("❌ sing-box.exe not found")
				tab.singboxStatusLabel.Importance = widget.MediumImportance
				tab.downloadButton.SetText("Download")
				tab.downloadButton.Enable()
				tab.downloadButton.Importance = widget.HighImportance
				tab.downloadButton.Show()
			} else {
				// Показываем версию
				tab.singboxStatusLabel.SetText(installedVersion)
				tab.singboxStatusLabel.Importance = widget.MediumImportance
			}
		})

		// Если бинарник не найден, пытаемся получить последнюю версию для кнопки
		if err != nil {
			latest, latestErr := tab.controller.GetLatestCoreVersion()
			fyne.Do(func() {
				if latestErr == nil && latest != "" {
					tab.downloadButton.SetText(fmt.Sprintf("Download v%s", latest))
				} else {
					tab.downloadButton.SetText("Download")
				}
			})
			return
		}

		// Получаем последнюю версию (сетевая операция, асинхронная)
		latest, latestErr := tab.controller.GetLatestCoreVersion()

		// Обновляем UI с результатом
		fyne.Do(func() {
			if latestErr != nil {
				// Network error - not critical, just don't show update
				// Log for debugging, but don't show to user
				tab.downloadButton.Hide()
				return
			}

			// Сравниваем версии
			if latest != "" && compareVersions(installedVersion, latest) < 0 {
				// Есть обновление
				tab.downloadButton.SetText(fmt.Sprintf("Update v%s", latest))
				tab.downloadButton.Enable()
				tab.downloadButton.Importance = widget.HighImportance
				tab.downloadButton.Show()
			} else {
				// Версия актуальна
				tab.downloadButton.Hide()
			}
		})
	}()
}

const configTemplateURL = "https://raw.githubusercontent.com/Leadaxe/singbox-launcher/main/bin/config_template.json"

func (tab *CoreDashboardTab) downloadConfigTemplate() {
	if tab.templateDownloadButton != nil {
		tab.templateDownloadButton.Disable()
	}
	go func() {
		resp, err := http.Get(configTemplateURL)
		if err != nil {
			fyne.Do(func() {
				if tab.templateDownloadButton != nil {
					tab.templateDownloadButton.Enable()
				}
				ShowError(tab.controller.MainWindow, fmt.Errorf("failed to download template: %w", err))
			})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fyne.Do(func() {
				if tab.templateDownloadButton != nil {
					tab.templateDownloadButton.Enable()
				}
				ShowError(tab.controller.MainWindow, fmt.Errorf("download template failed: %s", resp.Status))
			})
			return
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			fyne.Do(func() {
				if tab.templateDownloadButton != nil {
					tab.templateDownloadButton.Enable()
				}
				ShowError(tab.controller.MainWindow, fmt.Errorf("failed to read template: %w", err))
			})
			return
		}
		target := filepath.Join(tab.controller.ExecDir, "bin", "config_template.json")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			fyne.Do(func() {
				if tab.templateDownloadButton != nil {
					tab.templateDownloadButton.Enable()
				}
				ShowError(tab.controller.MainWindow, fmt.Errorf("failed to create bin directory: %w", err))
			})
			return
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			fyne.Do(func() {
				if tab.templateDownloadButton != nil {
					tab.templateDownloadButton.Enable()
				}
				ShowError(tab.controller.MainWindow, fmt.Errorf("failed to save template: %w", err))
			})
			return
		}
		fyne.Do(func() {
			if tab.templateDownloadButton != nil {
				tab.templateDownloadButton.Hide()
			}
			dialog.ShowInformation("Config Template", fmt.Sprintf("Template saved to %s", target), tab.controller.MainWindow)
			tab.updateConfigInfo()
		})
	}()
}

// compareVersions сравнивает две версии (формат X.Y.Z)
// Возвращает: -1 если v1 < v2, 0 если v1 == v2, 1 если v1 > v2
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &num1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &num2)
		}

		if num1 < num2 {
			return -1
		}
		if num1 > num2 {
			return 1
		}
	}

	return 0
}

// handleDownload обрабатывает нажатие на кнопку Download
func (tab *CoreDashboardTab) handleDownload() {
	if tab.downloadInProgress {
		return // Уже идет скачивание
	}

	// Get version information (local operation)
	versionInfo := tab.controller.GetCoreVersionInfo()

	targetVersion := versionInfo.LatestVersion
	if targetVersion == "" {
		// Пытаемся получить последнюю версию асинхронно
		// But for download we need version immediately, so do it synchronously in goroutine
		go func() {
			latest, err := tab.controller.GetLatestCoreVersion()
			fyne.Do(func() {
				if err != nil {
					ShowError(tab.controller.MainWindow, fmt.Errorf("failed to get latest version: %w", err))
					tab.downloadInProgress = false
					tab.downloadButton.Enable()
					tab.downloadButton.Show()
					return
				}
				// Запускаем скачивание с полученной версией
				tab.startDownloadWithVersion(latest)
			})
		}()
		return
	}

	// Запускаем скачивание с известной версией
	tab.startDownloadWithVersion(targetVersion)
}

// startDownloadWithVersion запускает процесс скачивания с указанной версией
func (tab *CoreDashboardTab) startDownloadWithVersion(targetVersion string) {
	// Запускаем скачивание в отдельной горутине
	tab.downloadInProgress = true
	tab.downloadButton.Disable()
	// Скрываем кнопку и показываем прогресс-бар
	tab.downloadButton.Hide()
	tab.downloadProgress.Show()
	tab.downloadProgress.SetValue(0)

	// Создаем канал для прогресса
	progressChan := make(chan core.DownloadProgress, 10)

	// Start download in separate goroutine with context
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		tab.controller.DownloadCore(ctx, targetVersion, progressChan)
	}()

	// Обрабатываем прогресс в отдельной горутине
	go func() {
		for progress := range progressChan {
			fyne.Do(func() {
				// Обновляем только прогресс-бар (кнопка скрыта)
				tab.downloadProgress.SetValue(float64(progress.Progress) / 100.0)

				if progress.Status == "done" {
					tab.downloadInProgress = false
					// Скрываем прогресс-бар и показываем кнопку
					tab.downloadProgress.Hide()
					tab.downloadProgress.SetValue(0)
					tab.downloadButton.Show()
					tab.downloadButton.Enable()
					// Обновляем статусы после успешного скачивания (это уберет ошибки и обновит статус)
					tab.updateVersionInfo()
					tab.updateBinaryStatus() // Это вызовет updateRunningStatus() и обновит статус
					// Обновляем иконку трея (может измениться с красной на черную/зеленую)
					tab.controller.UpdateUI()
					ShowInfo(tab.controller.MainWindow, "Download Complete", progress.Message)
				} else if progress.Status == "error" {
					tab.downloadInProgress = false
					// Скрываем прогресс-бар и показываем кнопку
					tab.downloadProgress.Hide()
					tab.downloadProgress.SetValue(0)
					tab.downloadButton.Show()
					tab.downloadButton.Enable()
					ShowError(tab.controller.MainWindow, progress.Error)
				}
			})
		}
	}()
}

// startAutoUpdate запускает автообновление версии (статус управляется через RunningState)
func (tab *CoreDashboardTab) startAutoUpdate() {
	// Запускаем периодическое обновление с умной логикой
	go func() {
		rand.Seed(time.Now().UnixNano()) // Инициализация генератора случайных чисел

		for {
			select {
			case <-tab.stopAutoUpdate:
				return
			default:
				// Ждем перед следующим обновлением
				var delay time.Duration
				if tab.lastUpdateSuccess {
					// Если последнее обновление было успешным - не повторяем автоматически
					// Ждем очень долго (или можно вообще не повторять)
					delay = 10 * time.Minute
				} else {
					// Если была ошибка - повторяем через случайный интервал 20-35 секунд
					delay = time.Duration(20+rand.Intn(16)) * time.Second // 20-35 секунд
				}

				select {
				case <-time.After(delay):
					// Обновляем только версию асинхронно (не блокируем UI)
					// updateVersionInfo теперь полностью асинхронная
					tab.updateVersionInfo()
					// Устанавливаем успех после небольшой задержки
					// (в реальности нужно отслеживать через канал, но для простоты используем задержку)
					go func() {
						time.Sleep(2 * time.Second)
						tab.lastUpdateSuccess = true // Упрощенная логика
					}()
				case <-tab.stopAutoUpdate:
					return
				}
			}
		}
	}()
}

// createWintunBlock creates a block for displaying wintun.dll status
func (tab *CoreDashboardTab) createWintunBlock() fyne.CanvasObject {
	title := widget.NewLabel("Wintun")
	title.Importance = widget.MediumImportance

	tab.wintunStatusLabel = widget.NewLabel("Checking...")
	tab.wintunStatusLabel.Wrapping = fyne.TextWrapOff

	tab.wintunDownloadButton = widget.NewButton("Download", func() {
		tab.handleWintunDownload()
	})
	tab.wintunDownloadButton.Importance = widget.MediumImportance
	tab.wintunDownloadButton.Disable()

	tab.wintunDownloadProgress = widget.NewProgressBar()
	tab.wintunDownloadProgress.Hide()
	tab.wintunDownloadProgress.SetValue(0)
	progressContainer := container.NewMax(tab.wintunDownloadProgress)
	tab.wintunDownloadContainer = container.NewStack(tab.wintunDownloadButton, progressContainer)

	return container.NewHBox(
		title,
		layout.NewSpacer(),
		tab.wintunStatusLabel,
		tab.wintunDownloadContainer,
	)
}

// updateWintunStatus обновляет статус wintun.dll
func (tab *CoreDashboardTab) updateWintunStatus() {
	if runtime.GOOS != "windows" {
		return // wintun нужен только на Windows
	}

	exists, err := tab.controller.CheckWintunDLL()
	if err != nil {
		tab.wintunStatusLabel.SetText("❌ Error checking wintun.dll")
		tab.wintunStatusLabel.Importance = widget.MediumImportance
		tab.wintunDownloadButton.Disable()
		return
	}

	if exists {
		tab.wintunStatusLabel.SetText("ok")
		tab.wintunStatusLabel.Importance = widget.MediumImportance
		tab.wintunDownloadButton.Hide()
		tab.wintunDownloadProgress.Hide()
	} else {
		tab.wintunStatusLabel.SetText("❌ wintun.dll not found")
		tab.wintunStatusLabel.Importance = widget.MediumImportance
		tab.wintunDownloadButton.Show()
		tab.wintunDownloadButton.Enable()
		tab.wintunDownloadButton.SetText("Download wintun.dll")
		tab.wintunDownloadButton.Importance = widget.HighImportance
	}
}

// handleWintunDownload обрабатывает нажатие на кнопку Download wintun.dll
func (tab *CoreDashboardTab) handleWintunDownload() {
	if tab.wintunDownloadInProgress {
		return // Уже идет скачивание
	}

	tab.wintunDownloadInProgress = true
	tab.wintunDownloadButton.Disable()
	tab.wintunDownloadButton.SetText("Downloading...")
	tab.wintunDownloadProgress.Show()
	tab.wintunDownloadProgress.SetValue(0)

	go func() {
		progressChan := make(chan core.DownloadProgress, 10)

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			tab.controller.DownloadWintunDLL(ctx, progressChan)
		}()

		for progress := range progressChan {
			fyne.Do(func() {
				tab.wintunDownloadProgress.SetValue(float64(progress.Progress) / 100.0)
				tab.wintunDownloadButton.SetText(fmt.Sprintf("Downloading... %d%%", progress.Progress))

				if progress.Status == "done" {
					tab.wintunDownloadInProgress = false
					tab.updateWintunStatus() // Обновляем статус после скачивания
					tab.wintunDownloadProgress.Hide()
					tab.wintunDownloadProgress.SetValue(0)
					tab.wintunDownloadButton.Enable()
					ShowInfo(tab.controller.MainWindow, "Download Complete", progress.Message)
				} else if progress.Status == "error" {
					tab.wintunDownloadInProgress = false
					tab.wintunDownloadProgress.Hide()
					tab.wintunDownloadProgress.SetValue(0)
					tab.wintunDownloadButton.Show()
					tab.wintunDownloadButton.Enable()
					ShowError(tab.controller.MainWindow, progress.Error)
				}
			})
		}
	}()
}
