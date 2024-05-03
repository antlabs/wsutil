#include <immintrin.h>
#include <stdint.h>

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

// 一次处理128字节
static INLINE void mask_avx2_128(uint8_t *payload, size_t size, __m256i key256) {
    __m256i data0 = _mm256_loadu_si256(((__m256i *)payload) + 0);
    __m256i data1 = _mm256_loadu_si256(((__m256i *)payload) + 1);
    __m256i data2 = _mm256_loadu_si256(((__m256i *)payload) + 2);
    __m256i data3 = _mm256_loadu_si256(((__m256i *)payload) + 3);

    __m256i result0 = _mm256_xor_si256(data0, key256);
    __m256i result1 = _mm256_xor_si256(data1, key256);
    __m256i result2 = _mm256_xor_si256(data2, key256);
    __m256i result3 = _mm256_xor_si256(data3, key256);

    _mm256_storeu_si256((((__m256i *)payload)+ 0), result0);
    _mm256_storeu_si256((((__m256i *)payload)+ 1), result1);
    _mm256_storeu_si256((((__m256i *)payload)+ 2), result2);
    _mm256_storeu_si256((((__m256i *)payload)+ 3), result3);
}

// 一次处理64字节
static INLINE void mask_avx2_64(uint8_t *payload, size_t size, __m256i key256) {
    __m256i data0 = _mm256_loadu_si256(((__m256i *)payload) + 0);
    __m256i data1 = _mm256_loadu_si256(((__m256i *)payload) + 1);

    __m256i result0 = _mm256_xor_si256(data0, key256);
    __m256i result1 = _mm256_xor_si256(data1, key256);

    _mm256_storeu_si256((((__m256i *)payload)+ 0), result0);
    _mm256_storeu_si256((((__m256i *)payload)+ 1), result1);
}

// 一次处理32字节
static INLINE void mask_avx2_32(uint8_t *payload, size_t size, __m256i key256) {
    __m256i data0 = _mm256_loadu_si256(((__m256i *)payload) + 0);

    __m256i result0 = _mm256_xor_si256(data0, key256);

    _mm256_storeu_si256((((__m256i *)payload)+ 0), result0);
}

// 一次处理16字节
static INLINE void mask_avx2_16(uint8_t *payload, size_t size, uint32_t key) {
    __m128i data0 = _mm128_loadu_si128(((__m128i *)payload) + 0);

    __m128i result0 = _mm128_xor_si128(data0, key128);

    _mm128_storeu_si128((((__m128i *)payload)+ 0), result0);
}

// 一次处理8字节
static INLINE void mask_8(uint8_t *payload, size_t size, uint32_t key) {
    uint64_t key64 = key << 64 | key;
    *(*uit64_t)(payload) = key64;
}

// 1-255大小的处理
static INLINE void mask_tiny(uint8_t *payload, size_t size, uint32_t key, __m256i key256) {
    // Handle the remaining data
    size_t remaining = size & ~(size_t)0x1F;
    switch (size - remaining) {
        case 0: break;
        case 16: mask_avx2_128(payload + remaining, 16, key); break;
        case 32: mask_avx2(payload + remaining, 32, key); break;
        case 64: mask_avx2_64(payload + remaining, 64, key); break;
        case 128: mask_avx2_128(payload + remaining, 128, key); break;
        default: mask_tiny(payload + remaining, size - remaining, key);
    }
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

    if (size > 0) {
        mask_tiny(payload, size, key, key256);
    }
}