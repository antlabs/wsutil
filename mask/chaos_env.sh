#!/usr/bin/env bash
# Copyright 2021-2024 antlabs. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# 在多种运行时配置下跑 mask 的测试套件。
#
# 为什么需要: 常规 `go test` 只用一种运行时配置。汇编是 NOSPLIT 且操作
# 裸指针的, 它对调度器/GC/栈的假定在某些配置下才会暴露问题 ——
# 例如关闭抢占、改变栈收缩策略、极端 GC 频率。
# 这些配置组合很难靠人工记得每次都跑, 所以固化成脚本。
#
# 用法:
#   ./chaos_env.sh            # 跑全部变体
#   ./chaos_env.sh -v         # 显示每个变体的完整输出
#
# 只在 darwin/arm64 上有意义(其余平台 wsutil_neon 构建约束不生效)。

set -u

cd "$(dirname "$0")/.." || exit 1

VERBOSE=0
[ "${1:-}" = "-v" ] && VERBOSE=1

PKG="./mask/"
TAGS="-tags wsutil_neon"
PASS=0
FAIL=0
FAILED_NAMES=()

# run <描述> <env赋值...> -- <额外 go test 参数...>
run() {
    local desc="$1"; shift
    local envs=()
    while [ $# -gt 0 ] && [ "$1" != "--" ]; do
        envs+=("$1"); shift
    done
    [ "${1:-}" = "--" ] && shift

    # 注意: 没有环境变量时不能写 env "" go test ...,
    # 那会让 env 把空串当命令名, 报 "No such file or directory"。
    local out
    if [ ${#envs[@]} -eq 0 ]; then
        out=$(go test $TAGS "$PKG" -count=1 -timeout=600s "$@" 2>&1)
    else
        out=$(env "${envs[@]}" go test $TAGS "$PKG" -count=1 -timeout=600s "$@" 2>&1)
    fi
    if echo "$out" | tail -1 | grep -q '^ok'; then
        printf '  [OK]   %s\n' "$desc"
        PASS=$((PASS+1))
    else
        printf '  [FAIL] %s\n' "$desc"
        FAIL=$((FAIL+1))
        FAILED_NAMES+=("$desc")
        echo "$out" | grep -vE '^\s*$' | head -6 | sed 's/^/         /'
    fi
    [ "$VERBOSE" = "1" ] && echo "$out" | tail -3 | sed 's/^/         /'
    return 0
}

if [ "$(go env GOOS)" != "darwin" ] || [ "$(go env GOARCH)" != "arm64" ]; then
    echo "跳过: 当前平台 $(go env GOOS)/$(go env GOARCH), chaos_env.sh 仅在 darwin/arm64 有意义"
    exit 0
fi

echo "=== 构建配置 ==="
# -N -l 关闭优化与内联, 会改变 Go 侧的寄存器分配和调用点的栈布局
# 注意: -gcflags 的值必须整体加引号。写成 -gcflags=-N -l 时,
# shell 会把 -l 当成 go test 自己的参数, go 静默忽略它,
# 结果是"以为测了无优化模式, 实际没测"。
run "关闭优化 (-gcflags=-N -l)" -- '-gcflags=-N -l'
# 竞争检测本身会改变调度时序
run "-race" -- -race
# 覆盖统计会增加额外的写屏障/计数
run "覆盖统计 (-cover)" -- -cover

echo
echo "=== GODEBUG: 调度与抢占 ==="
run "asyncpreemptoff=1  关闭异步抢占"        GODEBUG=asyncpreemptoff=1
run "preemptibleloops=1 循环可抢占"          GODEBUG=preemptibleloops=1
run "gccheckmark=1        GC 标记校验"       GODEBUG=gccheckmark=1
run "gcshrinkstackoff=1   不收缩栈"          GODEBUG=gcshrinkstackoff=1

echo
echo "=== GODEBUG: GC 行为 ==="
run "gcstoptheworld=1     每次 GC 全停"      GODEBUG=gcstoptheworld=1
run "gcstoptheworld=2     GC 后不重启世界"   GODEBUG=gcstoptheworld=2
run "clobberfree=1        释放内存填垃圾"    GODEBUG=clobberfree=1
run "madvdontneed=1       立即归还内存"      GODEBUG=madvdontneed=1

echo
echo "=== GOGC 频率 ==="
run "GOGC=off             关闭 GC"            GOGC=off
run "GOGC=1               极端频繁 GC"       GOGC=1
run "GOGC=1000            极少 GC"           GOGC=1000

echo
echo "=== GOEXPERIMENT (编译期) ==="
# cgocheck2 在新版 Go 里只能编译期开启, 运行期 GODEBUG=cgocheck=2 会直接 fatal
run "GOEXPERIMENT=cgocheck2  指针类型严格校验" GOEXPERIMENT=cgocheck2
run "GOEXPERIMENT=arenas     实验性内存分配"   GOEXPERIMENT=arenas

echo
echo "=== GOMAXPROCS ==="
for n in 1 2 4; do
    run "GOMAXPROCS=$n" GOMAXPROCS=$n
done

echo
echo "============================================"
printf '通过 %d 项, 失败 %d 项\n' "$PASS" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
    echo "失败项:"
    for n in "${FAILED_NAMES[@]}"; do echo "  - $n"; done
    exit 1
fi
echo "全部通过"
