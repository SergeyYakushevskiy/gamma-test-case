package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testStrings []string

func init() {
	var textPath string

	textPath = filepath.Join("testdata", "lorem_ipsum.txt")
	content, err := os.ReadFile(textPath)
	if err != nil {
		panic(fmt.Sprintf("Не удалось прочитать тестовый файл: %v", err))
	}

	testStrings = strings.Fields(string(content))
	if len(testStrings) == 0 {
		panic("Тестовый файл не содержит слов")
	}
	log.Printf("Загружено %d слов из файла %s", len(testStrings), textPath)
}

// Конкатенация строк через оператор "+"
func BenchmarkConcatPlus(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var s string
		for _, v := range testStrings {
			s += v
		}
		_ = s
	}
}

// Конкатенация строк через fmt.Sprintf
func BenchmarkConcatSprintf(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var s string
		for _, v := range testStrings {
			s = fmt.Sprintf("%s%s", s, v)
		}
		_ = s
	}
}

// Конкатенация строк через strings.Join
func BenchmarkConcatStringsJoin(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := strings.Join(testStrings, "")
		_ = s
	}
}

// Конкатенация строк через strings.Builder без выделения памяти
func BenchmarkConcatStringBuilder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var builder strings.Builder
		for _, v := range testStrings {
			builder.WriteString(v)
		}
		s := builder.String()
		_ = s
	}
}

// Конкатенация строк через strings.Builder с предварительным выделением памяти
func BenchmarkConcatStringBuilderGrow(b *testing.B) {
	// Рассчёт точной длины
	totalLen := 0
	for _, v := range testStrings {
		totalLen += len(v)
	}

	b.ResetTimer() // Сброс таймера, чтобы расчет длины не шел в зачет скорости
	for i := 0; i < b.N; i++ {
		var builder strings.Builder
		builder.Grow(totalLen)
		for _, v := range testStrings {
			builder.WriteString(v)
		}
		s := builder.String()
		_ = s
	}
}
