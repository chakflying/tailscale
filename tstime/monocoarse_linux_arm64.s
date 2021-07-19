// Copyright (c) 2021 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Adapted from code in the Go runtime package at Go 1.16.6:

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.16
// +build !go1.17

#include "go_asm.h"
#include "textflag.h"

#define SYS_clock_gettime   113

// Hard-coded offsets into runtime structs.
// Generated by adding the following code
// to package runtime and then executing
// and empty func main:
//
// func init() {
//	println("#define g_m", unsafe.Offsetof(g{}.m))
//	println("#define g_sched", unsafe.Offsetof(g{}.sched))
//	println("#define gobuf_sp", unsafe.Offsetof(g{}.sched.sp))
//	println("#define g_stack", unsafe.Offsetof(g{}.stack))
//	println("#define stack_lo", unsafe.Offsetof(g{}.stack.lo))
//	println("#define m_g0", unsafe.Offsetof(m{}.g0))
//	println("#define m_curg", unsafe.Offsetof(m{}.curg))
//	println("#define m_vdsoSP", unsafe.Offsetof(m{}.vdsoSP))
//	println("#define m_vdsoPC", unsafe.Offsetof(m{}.vdsoPC))
//	println("#define m_gsignal", unsafe.Offsetof(m{}.gsignal))
// }

#define g_m 48
#define g_sched 56
#define gobuf_sp 0
#define g_stack 0
#define stack_lo 0
#define m_g0 0
#define m_curg 192
#define m_vdsoSP 832
#define m_vdsoPC 840
#define m_gsignal 80

#define CLOCK_MONOTONIC	1

// func MonotonicCoarse() int64
TEXT ·MonotonicCoarse(SB),NOSPLIT,$24-8
	MOVD	RSP, R20	// R20 is unchanged by C code
	MOVD	RSP, R1

	MOVD	g_m(g), R21	// R21 = m

	// Set vdsoPC and vdsoSP for SIGPROF traceback.
	// Save the old values on stack and restore them on exit,
	// so this function is reentrant.
	MOVD	m_vdsoPC(R21), R2
	MOVD	m_vdsoSP(R21), R3
	MOVD	R2, 8(RSP)
	MOVD	R3, 16(RSP)

	MOVD	LR, m_vdsoPC(R21)
	MOVD	R20, m_vdsoSP(R21)

	MOVD	m_curg(R21), R0
	CMP	g, R0
	BNE	noswitch

	MOVD	m_g0(R21), R3
	MOVD	(g_sched+gobuf_sp)(R3), R1	// Set RSP to g0 stack

noswitch:
	SUB	$32, R1
	BIC	$15, R1
	MOVD	R1, RSP

	MOVW	$CLOCK_MONOTONIC, R0
	MOVD	runtime·vdsoClockgettimeSym(SB), R2
	CBZ	R2, fallback

	// Store g on gsignal's stack, so if we receive a signal
	// during VDSO code we can find the g.
	// If we don't have a signal stack, we won't receive signal,
	// so don't bother saving g.
	// When using cgo, we already saved g on TLS, also don't save
	// g here.
	// Also don't save g if we are already on the signal stack.
	// We won't get a nested signal.
	MOVBU	runtime·iscgo(SB), R22
	CBNZ	R22, nosaveg
	MOVD	m_gsignal(R21), R22          // g.m.gsignal
	CBZ	R22, nosaveg
	CMP	g, R22
	BEQ	nosaveg
	MOVD	(g_stack+stack_lo)(R22), R22 // g.m.gsignal.stack.lo
	MOVD	g, (R22)

	BL	(R2)

	MOVD	ZR, (R22)  // clear g slot, R22 is unchanged by C code

	B	finish

nosaveg:
	BL	(R2)
	B	finish

fallback:
	MOVD	$SYS_clock_gettime, R8
	SVC

finish:
	MOVD	0(RSP), R3	// sec
	MOVD	8(RSP), R5	// nsec

	MOVD	R20, RSP	// restore SP
	// Restore vdsoPC, vdsoSP
	// We don't worry about being signaled between the two stores.
	// If we are not in a signal handler, we'll restore vdsoSP to 0,
	// and no one will care about vdsoPC. If we are in a signal handler,
	// we cannot receive another signal.
	MOVD	16(RSP), R1
	MOVD	R1, m_vdsoSP(R21)
	MOVD	8(RSP), R1
	MOVD	R1, m_vdsoPC(R21)

	// sec is in R3, nsec in R5
	// return nsec in R3
	MOVD	$1000000000, R4
	MUL	R4, R3
	ADD	R5, R3
	MOVD	R3, ret+0(FP)
	RET
