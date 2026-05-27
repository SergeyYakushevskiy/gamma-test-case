package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

type Config struct {
	WorkerCount int
	ColA        string
	ColB        string
	Threshold   float64
}

// RowData переносит данные строки через конвейер
type RowData struct {
	ID             int
	Record         []string
	ValA           float64
	ValB           float64
	Sum            float64
	BelowThreshold bool
	Err            error
}

type Stats struct {
	TotalRows      int
	TotalSum       float64
	BelowThreshold int
}

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("Использование: %s <config.json> <input.csv> <output.csv>", os.Args[0])
	}

	configPath := os.Args[1]
	inputPath := os.Args[2]
	outputPath := os.Args[3]

	config, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Ошибка загрузки конфига: %v", err)
	}

	inFile, err := os.Open(inputPath)
	if err != nil {
		log.Fatalf("Ошибка открытия входного файла: %v", err)
	}
	defer inFile.Close()

	outFile, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("Ошибка создания выходного файла: %v", err)
	}
	defer outFile.Close()

	reader := csv.NewReader(inFile)
	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	header, err := reader.Read()
	if err != nil {
		log.Fatalf("Ошибка чтения заголовка CSV: %v", err)
	}

	indexA, indexB := -1, -1
	for i, col := range header {
		if col == config.ColA {
			indexA = i
		}
		if col == config.ColB {
			indexB = i
		}
	}
	if indexA == -1 || indexB == -1 {
		log.Fatalf("Колонки %s или %s не найдены в CSV", config.ColA, config.ColB)
	}

	newHeader := append(header, "sum", "below_threshold")
	if err := writer.Write(newHeader); err != nil {
		log.Fatalf("Ошибка записи заголовка: %v", err)
	}

	// Каналы для связи этапов
	jobsChan := make(chan RowData, config.WorkerCount*2)
	resultsChan := make(chan RowData, config.WorkerCount*2)

	go func() {
		defer close(jobsChan)
		id := 0
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				jobsChan <- RowData{ID: id, Err: err}
				break
			}

			// Считывание сырых строк для обработки воркерами
			// csv.Reader может переиспользовать внутренний буфер
			recordCopy := make([]string, len(record))
			copy(recordCopy, record)

			var valA, valB float64
			if indexA < len(recordCopy) {
				valA, _ = strconv.ParseFloat(recordCopy[indexA], 64)
			}
			if indexB < len(recordCopy) {
				valB, _ = strconv.ParseFloat(recordCopy[indexB], 64)
			}

			jobsChan <- RowData{
				ID:     id,
				Record: recordCopy,
				ValA:   valA,
				ValB:   valB,
			}
			id++
		}
	}()

	workersDone := make(chan struct{})
	go func() {
		activeWorkers := config.WorkerCount
		doneSignal := make(chan struct{}, config.WorkerCount)

		for worker := 0; worker < config.WorkerCount; worker++ {
			go func() {
				defer func() { doneSignal <- struct{}{} }()
				for job := range jobsChan {
					if job.Err != nil {
						resultsChan <- job
						continue
					}
					// Логика обработки
					job.Sum = job.ValA + job.ValB
					job.BelowThreshold = job.Sum < config.Threshold
					resultsChan <- job
				}
			}()
		}

		for i := 0; i < activeWorkers; i++ {
			<-doneSignal
		}
		close(resultsChan)
		close(workersDone)
	}()

	stats := Stats{}

	orderBuffer := make(map[int]RowData)
	expectedID := 0

	for result := range resultsChan {
		if result.Err != nil {
			log.Printf("Ошибка обработки строки ID %d: %v", result.ID, result.Err)
			continue
		}

		orderBuffer[result.ID] = result

		// Считывание из буфера всего, что готово к записи по порядку цепочки
		for {
			row, exists := orderBuffer[expectedID]
			if !exists {
				break // Следующий по порядку элемент еще не обработан воркерами
			}

			delete(orderBuffer, expectedID)

			stats.TotalRows++
			stats.TotalSum += row.Sum
			if row.BelowThreshold {
				stats.BelowThreshold++
			}

			outputRecord := append(row.Record,
				strconv.FormatFloat(row.Sum, 'f', -1, 64),
				strconv.FormatBool(row.BelowThreshold),
			)

			if err := writer.Write(outputRecord); err != nil {
				log.Fatalf("Ошибка записи в выходной файл: %v", err)
			}

			expectedID++
		}
	}

	<-workersDone

	// JSON-отчет в stdout
	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		log.Fatalf("Ошибка генерации JSON-отчета: %v", err)
	}
	fmt.Println(string(statsJSON))
}

func loadConfig(path string) (Config, error) {
	var config Config
	file, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&config)
	return config, err
}
