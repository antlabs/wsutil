test:
	go test ./...
	go test -tags=goexperiment.arenas ./...

build-asm:
	- mkdir output
	# goat mask/native/mask_avx2.c -O3 -mavx -mfma -mavx512f -mavx512dq
	# clang -mno-red-zone -fno-asynchronous-unwind-tables -fno-builtin -fno-exceptions -fno-rtti -fno-stack-protector -nostdlib -O3 -Wall -Werror -msse -mno-sse4 -mavx -mavx2 -DUSE_AVX=1 -DUSE_AVX2=1 -S -o output/mask_avx2_native.s mask/native/mask_avx2.c
	clang -mno-red-zone -fno-asynchronous-unwind-tables -fno-builtin -fno-exceptions -fno-rtti -fno-stack-protector -nostdlib -O3 -Wall -Werror -msse  -mno-sse4 -mavx -mavx2 -DUSE_AVX=1  -DUSE_AVX2=1 -S -o output/mask_avx2_native.s mask/native/mask_avx2.c
	# python3 ./asm2asm.py -r mask/mask_avx2_amd64.go output/mask_avx2_native.s 
