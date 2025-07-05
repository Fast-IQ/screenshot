package gdi

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestAVX2Conversion(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte // BGRx
		expected []byte // RGBx
	}{
		{
			"Blue to Red",
			[]byte{0xFF, 0x00, 0x00, 0xAA}, // BGRx
			[]byte{0x00, 0x00, 0xFF, 0xAA}, // RGBx
		},
		{
			"Green remains",
			[]byte{0x00, 0xFF, 0x00, 0xBB}, // BGRx
			[]byte{0x00, 0xFF, 0x00, 0xBB}, // RGBx
		},
		{
			"Red to Blue",
			[]byte{0x00, 0x00, 0xFF, 0xCC}, // BGRx
			[]byte{0xFF, 0x00, 0x00, 0xCC}, // RGBx
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := make([]byte, len(tt.input))
			swapBGRtoRGB_AVX2(tt.input, dst)

			if !bytes.Equal(dst, tt.expected) {
				t.Errorf("got %v, want %v", dst, tt.expected)
			}
		})
	}
}

func BenchmarkConversion(b *testing.B) {
	// Подготовка тестовых данных (4K изображение)
	data := make([]byte, 3840*2160*4)
	rand.Read(data)
	dst := make([]byte, len(data))

	b.Run("AVX2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			swapBGRtoRGB_AVX2(data, dst)
		}
	})

	b.Run("Go", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			swapBGRtoRGB_Go(data, dst)
		}
	})
}

func TestConversionAccuracy(t *testing.T) {
	// Создаем тестовое изображение 4x4 пикселя
	width, height := 4, 4
	src := make([]byte, width*height*4)

	// Заполняем предсказуемыми данными
	for i := 0; i < len(src); i += 4 {
		src[i] = byte(i)       // B
		src[i+1] = byte(i + 1) // G
		src[i+2] = byte(i + 2) // R
		src[i+3] = 0xFF        // A
	}

	// Эталонное преобразование
	ref := make([]byte, len(src))
	swapBGRtoRGB_Go(src, ref)

	// AVX2 преобразование
	avx2 := make([]byte, len(src))
	swapBGRtoRGB_AVX2(src, avx2)

	// Построчное сравнение
	for i := 0; i < len(src); i += 4 {
		if avx2[i] != ref[i] || avx2[i+1] != ref[i+1] ||
			avx2[i+2] != ref[i+2] || avx2[i+3] != ref[i+3] {
			t.Errorf("Пиксель %d:\nAVX2: [%d %d %d %d]\nGo:   [%d %d %d %d]",
				i/4,
				avx2[i], avx2[i+1], avx2[i+2], avx2[i+3],
				ref[i], ref[i+1], ref[i+2], ref[i+3])
		}
	}

	// Дополнительный вывод для отладки
	t.Log("Первые 4 пикселя исходные (BGRx):", src[:16])
	t.Log("AVX2 результат:", avx2[:16])
	t.Log("Go результат:", ref[:16])
}

func TestSinglePixel(t *testing.T) {
	src := []byte{0x00, 0x01, 0x02, 0xFF} // BGRx
	dst := make([]byte, 4)

	swapBGRtoRGB_AVX2(src, dst)

	if !bytes.Equal(dst, []byte{0x02, 0x01, 0x00, 0xFF}) {
		t.Errorf("Ожидается [2 1 0 255], получен %v", dst)
	}
}

func TestTwoPixels(t *testing.T) {
	src := []byte{0x00, 0x01, 0x02, 0xFF, 0x04, 0x05, 0x06, 0xFF} // BGRx BGRx
	dst := make([]byte, 8)

	swapBGRtoRGB_AVX2(src, dst)

	if !bytes.Equal(dst, []byte{0x02, 0x01, 0x00, 0xFF, 0x06, 0x05, 0x04, 0xFF}) {
		t.Errorf("Ожидается [2 1 0 255 6 5 4 255], получен %v", dst)
	}
}

func TestFourPixels(t *testing.T) {
	src := []byte{
		0x00, 0x01, 0x02, 0xFF, // BGRx
		0x04, 0x05, 0x06, 0xFF, // BGRx
		0x08, 0x09, 0x0A, 0xFF, // BGRx
		0x0C, 0x0D, 0x0E, 0xFF, // BGRx
	}
	dst := make([]byte, 16)

	swapBGRtoRGB_AVX2(src, dst)

	if !bytes.Equal(dst, []byte{
		0x02, 0x01, 0x00, 0xFF, // RGBx
		0x06, 0x05, 0x04, 0xFF, // RGBx
		0x0A, 0x09, 0x08, 0xFF, // RGBx
		0x0E, 0x0D, 0x0C, 0xFF, // RGBx
	}) {
		t.Errorf("Ожидается [2 1 0 255 ...], получен %v", dst)
	}
}

func TestShuffleMask(t *testing.T) {
	// Тестовые данные - 4 пикселя BGRx
	in := []byte{
		0x00, 0x01, 0x02, 0xFF, // Пиксель 0: B=0x00, G=0x01, R=0x02, A=0xFF
		0x04, 0x05, 0x06, 0xFF, // Пиксель 1
		0x08, 0x09, 0x0A, 0xFF, // Пиксель 2
		0x0C, 0x0D, 0x0E, 0xFF, // Пиксель 3
	}

	// Ожидаемый результат после преобразования
	want := []byte{
		0x02, 0x01, 0x00, 0xFF, // R, G, B, A
		0x06, 0x05, 0x04, 0xFF,
		0x0A, 0x09, 0x08, 0xFF,
		0x0E, 0x0D, 0x0C, 0xFF,
	}

	// Выводим информацию для отладки
	t.Logf("Input (BGRx):  % 02X", in)
	t.Logf("Want (RGBx):  % 02X", want)

	// Проверяем преобразование для каждого байта
	for i := 0; i < len(in); i += 4 {
		b, g, r, a := in[i], in[i+1], in[i+2], in[i+3]
		t.Logf("Pixel %d: B=%02X G=%02X R=%02X A=%02X => Want: R=%02X G=%02X B=%02X A=%02X",
			i/4, b, g, r, a, r, g, b, a)
	}
}
