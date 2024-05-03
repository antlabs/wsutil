#include <immintrin.h>
#include <emmintrin.h>
#include <stdint.h>
#include <assert.h>
#include <stddef.h>

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
    __m128i key128 = _mm_set1_epi32(key);
    __m128i data0 = _mm_loadu_si128(((__m128i *)payload) + 0);

    __m128i result0 = _mm_xor_si128(data0, key128);

    _mm_storeu_si128((((__m128i *)payload)+ 0), result0);
}

// 一次处理8字节
static INLINE void mask_avx2_8(uint8_t *payload, size_t size, uint32_t key) {
    uint64_t key64 = ((uint64_t)key) << 32| key;
    *(uint64_t *)(payload) = key64;
}

// 一次处理4字节
static INLINE void mask_avx2_4(uint8_t *payload, size_t size, uint32_t key) {
    *(uint32_t *)(payload) = key;
}

// 一次处理2字节
static INLINE void mask_avx2_2(uint8_t *payload, size_t size, uint32_t key) {
    *(uint16_t *)payload = (uint16_t)key;
}

// 一次处理1字节
static INLINE void mask_avx2_1(uint8_t *payload, size_t size, uint32_t key) {
    *payload = (uint8_t)key;
}

// 1-255大小的处理
static INLINE void mask_tiny(uint8_t *payload, size_t size, uint32_t key, __m256i key256) {
    switch (size) {
        case 0: break;
        case 128: mask_avx2_128(payload, size, key256); break;
        case 64: mask_avx2_64(payload, size, key256);
        case 32: mask_avx2_32(payload, size, key256); break;
        case 96: mask_avx2_64(payload, 64, key256);
                 mask_avx2_32(payload + 64, 32, key256); break;
        case 16: mask_avx2_16(payload, size, key); break;
        case 48: mask_avx2_32(payload, 32, key256);
                 mask_avx2_16(payload + 32, 16, key); break;
        case 80: mask_avx2_64(payload, 64, key256);
                 mask_avx2_16(payload + 64, 16, key); break;
        case 112: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key); break;
        case 8: mask_avx2_8(payload, size, key); break;
        case 24: mask_avx2_16(payload, 16, key);
                 mask_avx2_8(payload + 16, 8, key); break;
        case 40: mask_avx2_32(payload, 32, key256);
                 mask_avx2_8(payload + 32, 8, key); break;
        case 56: mask_avx2_32(payload, 32, key256);
                 mask_avx2_16(payload + 32, 16, key);
                 mask_avx2_8(payload + 48, 8, key); break;
        case 72: mask_avx2_64(payload, 64, key256);
                 mask_avx2_8(payload + 64, 8, key); break;
        case 88: mask_avx2_64(payload, 64, key256);
                 mask_avx2_16(payload + 64, 16, key);
                 mask_avx2_8(payload + 80, 8, key); break;
        case 104: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_8(payload + 96, 8, key); break;
        case 120: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_8(payload + 112, 8, key); break;
        case 4: mask_avx2_4(payload, size, key); break;
        case 20: mask_avx2_16(payload, 16, key);
                 mask_avx2_4(payload + 16, 4, key); break;
        case 36: mask_avx2_32(payload, 32, key256);
                 mask_avx2_4(payload + 32, 4, key); break;
        case 52: mask_avx2_32(payload, 32, key256);
                 mask_avx2_16(payload + 32, 16, key);
                 mask_avx2_4(payload + 48, 4, key); break;
        case 68: mask_avx2_64(payload, 64, key256);
                 mask_avx2_4(payload + 64, 4, key); break;
        case 84: mask_avx2_64(payload, 64, key256);
                 mask_avx2_16(payload + 64, 16, key);
                 mask_avx2_4(payload + 80, 4, key); break;
        case 100: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_4(payload + 96, 4, key); break;
        case 116: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_4(payload + 112, 4, key); break;
        case 124: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_8(payload + 112, 8, key);
                  mask_avx2_4(payload + 120, 4, key); break;
        case 2: mask_avx2_2(payload, size, key); break;
        case 18: mask_avx2_16(payload, 16, key);
                 mask_avx2_2(payload + 16, 2, key); break;
        case 34: mask_avx2_32(payload, 32, key256);
                 mask_avx2_2(payload + 32, 2, key); break;
        case 50: mask_avx2_32(payload, 32, key256);
                 mask_avx2_16(payload + 32, 16, key);
                 mask_avx2_2(payload + 48, 2, key); break;
        case 66: mask_avx2_64(payload, 64, key256);
                 mask_avx2_2(payload + 64, 2, key); break;
        case 82: mask_avx2_64(payload, 64, key256);
                 mask_avx2_16(payload + 64, 16, key);
                 mask_avx2_2(payload + 80, 2, key); break;
        case 98: mask_avx2_64(payload, 64, key256);
                 mask_avx2_32(payload + 64, 32, key256);
                 mask_avx2_2(payload + 96, 2, key); break;
        case 114: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_2(payload + 112, 2, key); break;
        case 122: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_8(payload + 112, 8, key);
                  mask_avx2_2(payload + 120, 2, key); break;
        case 1: mask_avx2_1(payload, size, key); break;
        case 17: mask_avx2_16(payload, 16, key);
                 mask_avx2_1(payload + 16, 1, key); break;
        case 33: mask_avx2_32(payload, 32, key256);
                 mask_avx2_1(payload + 32, 1, key); break;
        case 49: mask_avx2_32(payload, 32, key256);
                 mask_avx2_16(payload + 32, 16, key);
                 mask_avx2_1(payload + 48, 1, key); break;
        case 65: mask_avx2_64(payload, 64, key256);
                 mask_avx2_1(payload + 64, 1, key); break;
        case 81: mask_avx2_64(payload, 64, key256);
                 mask_avx2_16(payload + 64, 16, key);
                 mask_avx2_1(payload + 80, 1, key); break;
        case 97: mask_avx2_64(payload, 64, key256);
                 mask_avx2_32(payload + 64, 32, key256);
                 mask_avx2_1(payload + 96, 1, key); break;
        case 113: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_1(payload + 112, 1, key); break;
        case 121: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_8(payload + 112, 8, key);
                  mask_avx2_1(payload + 120, 1, key); break;
        case 123: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_8(payload + 112, 8, key);
                  mask_avx2_2(payload + 120, 2, key);
                  mask_avx2_1(payload + 122, 1, key); break;
        case 125: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_8(payload + 112, 8, key);
                  mask_avx2_4(payload + 120, 4, key);
                  mask_avx2_1(payload + 124, 1, key); break;
        case 127: mask_avx2_64(payload, 64, key256);
                  mask_avx2_32(payload + 64, 32, key256);
                  mask_avx2_16(payload + 96, 16, key);
                  mask_avx2_8(payload + 112, 8, key);
                  mask_avx2_4(payload + 120, 4, key);
                  mask_avx2_2(payload + 124, 2, key);
                  mask_avx2_1(payload + 126, 1, key); break;
        default:
            assert(0 && "Unsupported size in mask_tiny");
            // for (size_t i = 0; i < size; i++) {
            //     payload[i] ^= (uint8_t)key;
            // }
    }
}

void mask_avx2(uint8_t *payload, size_t size, uint32_t key) {
    __m256i key256 = _mm256_set1_epi32(key);

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