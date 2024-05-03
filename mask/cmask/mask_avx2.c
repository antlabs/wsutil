#include <immintrin.h>

#ifndef INLINE
#ifdef __GNUC__
#if (__GNUC__ > 3) || ((__GNUC__ == 3) && (__GNUC_MINOR__ >= 1))
    #define INLINE         __inline__ __attribute__((always_inline))
#else
    #define INLINE         __inline__
#endif
#elif defined(_MSC_VER)
	#define INLINE __forceinline
#elif (defined(__BORLANDC__) || defined(__WATCOMC__))
    #define INLINE __inline
#else
    #define INLINE 
#endif
#endif

static INLINE void mask_avx2_128(uint8_t *payload, size_t size, uint32_t key) {

}

static INLINE void mask_avx2_64(uint8_t *payload, size_t size, uint32_t key) {

}

static INLINE void mask_avx2_32(uint8_t *payload, size_t size, uint32_t key) {

}

static INLINE void mask_avx2_16(uint8_t *payload, size_t size, uint32_t key) {

}

static INLINE void mask_tiny(uint8_t *payload, size_t size, uint32_t key) {

}

void mask_avx2(uint8_t *payload, size_t size, uint32_t key) {
    __m256i key256 = _mm256_set1_epi32(key);
    size_t i = 0;

    // Unroll the loop 8 times
    for (; size >= 256; size -= 32 * 8) {
        __m256i data0 = _mm256_loadu_si256(((__m256i *)payload) + 0);
        __m256i data1 = _mm256_loadu_si256(((__m256i *)payload) + 1);
        __m256i data2 = _mm256_loadu_si256(((__m256i *)payload) + 2);
        __m256i data3 = _mm256_loadu_si256(((__m256i *)payload) + 3);
        __m256i data4 = _mm256_loadu_si256(((__m256i *)payload) + 4);
        __m256i data5 = _mm256_loadu_si256(((__m256i *)payload) + 5);
        __m256i data6 = _mm256_loadu_si256(((__m256i *)payload) + 6);
        __m256i data7 = _mm256_loadu_si256(((__m256i *)payload) + 7);

        __m256i result0 = _mm256_xor_si256(data0, key256);
        __m256i result1 = _mm256_xor_si256(data1, key256);
        __m256i result2 = _mm256_xor_si256(data2, key256);
        __m256i result3 = _mm256_xor_si256(data3, key256);
        __m256i result4 = _mm256_xor_si256(data4, key256);
        __m256i result5 = _mm256_xor_si256(data5, key256);
        __m256i result6 = _mm256_xor_si256(data6, key256);
        __m256i result7 = _mm256_xor_si256(data7, key256);

        _mm_prefetch((const char*)(payload + 512), _MM_HINT_NTA);

        _mm256_storeu_si256((((__m256i *)payload)+ 0), result0);
        _mm256_storeu_si256((((__m256i *)payload)+ 1), result1);
        _mm256_storeu_si256((((__m256i *)payload)+ 2), result2);
        _mm256_storeu_si256((((__m256i *)payload)+ 3), result3);
        _mm256_storeu_si256((((__m256i *)payload)+ 4), result4);
        _mm256_storeu_si256((((__m256i *)payload)+ 5), result5);
        _mm256_storeu_si256((((__m256i *)payload)+ 6), result6);
        _mm256_storeu_si256((((__m256i *)payload)+ 7), result7);
        payload += 256;
    }

    // Handle the remaining data
    for (; i < size; i += 32) {
        __m256i data = _mm256_loadu_si256((__m256i *)(payload + i));
        __m256i result = _mm256_xor_si256(data, key256);
        _mm256_storeu_si256((__m256i *)(payload + i), result);
    }
}