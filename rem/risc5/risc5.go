//
// Copyright © 2014 Peter De Wachter, 2017 Charles Perkins
//
// Permission to use, copy, modify, and/or distribute this software for any purpose with
// or without fee is hereby granted, provided that the above copyright notice and this
// permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO
// THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT
// SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR
// ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF
// CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE
// OR PERFORMANCE OF THIS SOFTWARE.
//



package risc5

import (
	"github.com/io-core/io/rem/board"
	"fmt"
)

//const ROMStart=    0xFFFFF800
//const ROMWords=    512

const pbit = 0x80000000
const qbit = 0x40000000
const ubit = 0x20000000
const vbit = 0x10000000


type CORE struct {
  Cid uint32
  PC uint32
  R [16]uint32
  H uint32
  Z, N, C, V, halt, pause bool

  OPC uint32
  OR [16]uint32
  OH uint32
  OZ, ON, OC, OV bool

  icount uint32
  progress uint32
  current_tick uint32
}


const(
  MOV = iota
  LSL
  ASR
  ROR
  AND
  ANN
  IOR
  XOR
  ADD
  SUB
  MUL
  DIV
  FAD
  FSB
  FML
  FDV
)


func (risc *CORE) Reset(n int, verbose bool) {
	risc.Cid=uint32(n)
        risc.icount=0
        risc.progress=20
	risc.PC=board.ROMStart/4
	if verbose {fmt.Printf("%s %d %s"," core ",int(risc.Cid)," reset ")}
}


func fp_add(x, y uint32, u, v bool) uint32{
  xs := (x & 0x80000000) != 0
  var xe uint32
  var x0 int32
  if (!u) {
    xe = (x >> 23) & 0xFF
    xm := uint32(((x & 0x7FFFFF) << 1) | 0x1000000)
    if xs {
      x0 = int32(-xm )
    }else{
      x0 = int32( xm )
    }
  } else {
    xe = 150
    x0 = int32((x & 0x00FFFFFF) << 8 >> 7)
  }

  ys := ((y & 0x80000000) != 0)
  ye := uint32((y >> 23) & 0xFF)
  ym := uint32(((y & 0x7FFFFF) << 1))
  if (!u && !v) { ym = ym | 0x1000000 }
  var y0 int32
  if ys {
    y0 = int32( -ym )
  }else{
    y0 = int32( ym )
  }

  var e0 uint32
  var x3, y3 int32
  if (ye > xe) {
    shift := uint32(ye - xe)
    e0 = ye
    if shift > 31 {
      x3 = x0 >> 31 
    }else{
      x3 = x0 >> shift
    }
    y3 = y0
  } else {
    shift := uint32(xe - ye)
    e0 = xe
    x3 = x0
    if shift > 31 {
      y3 =  y0 >> 31 
    }else{
      y3 =  y0 >> shift
    }
  }

  var sum1,sum2,sum uint32
  sum1=0
  sum2=0
  if xs {  sum1 = sum1 | uint32(1 << 26) }
  if xs {  sum1 = sum1 | uint32(1 << 25) }
  sum1 = sum1 | uint32(x3 & 0x01FFFFFF)
  if ys {  sum2 = sum2 | uint32(1 << 26) }
  if ys {  sum2 = sum2 | uint32(1 << 25) }
  sum2 = sum2 | uint32(y3 & 0x01FFFFFF)
  sum = sum1+sum2

//  sum := uint32(((xs << 26) | (xs << 25) | (x3 & 0x01FFFFFF)) + ((ys << 26) | (ys << 25) | (y3 & 0x01FFFFFF)))

  var s uint32
  if (sum & (1 << 26)) != 0 {
    s = uint32(( -sum + 1) & 0x07FFFFFF)
  }else{
    s = uint32((  sum + 1) & 0x07FFFFFF)
  }
  e1 := uint32(e0 + 1)
  t3 := uint32(s >> 1)
  if ((s & 0x3FFFFFC) != 0) {
    for ((t3 & (1<<24)) == 0) {
      t3 <<= 1
      e1--
    }
  }else{
    t3 <<= 24
    e1 -= 24
  }

  xn := (x & 0x7FFFFFFF) == 0
  yn := (y & 0x7FFFFFFF) == 0

  if (v) {
    return uint32((sum << 5) >> 6)
  } else if (xn) {
    if (u || yn) {
      return 0 
    }else{
      return y
    }
  } else if (yn) {
    return x
  } else if ((t3 & 0x01FFFFFF) == 0 || (e1 & 0x100) != 0) {
    return 0
  } else {
    return ((sum & 0x04000000) << 5) | (e1 << 23) | ((t3 >> 1) & 0x7FFFFF);
  }
}

func fp_mul(x, y uint32) uint32 {
  sign := uint32((x ^ y) & 0x80000000)
  xe := uint32((x >> 23) & 0xFF)
  ye := uint32((y >> 23) & 0xFF)

  xm := uint32((x & 0x7FFFFF) | 0x800000)
  ym := uint32((y & 0x7FFFFF) | 0x800000)
  m := uint64(uint64(xm) * uint64(ym))

  e1 := (xe + ye) - 127
  var z0 uint32
  if ((m & (uint64(1) << 47)) != 0) {
    e1++;
    z0 = uint32(((m >> 23) + 1) & 0xFFFFFF)
  } else {
    z0 = uint32(((m >> 22) + 1) & 0xFFFFFF)
  }

  if (xe == 0 || ye == 0) {
    return 0
  } else if ((e1 & 0x100) == 0) {
    return sign | ((e1 & 0xFF) << 23) | (z0 >> 1)
  } else if ((e1 & 0x80) == 0) {
    return sign | (0xFF << 23) | (z0 >> 1)
  } else {
    return 0
  }
}

func fp_div( x, y uint32) uint32 {
  sign := (x ^ y) & 0x80000000
  xe := (x >> 23) & 0xFF
  ye := (y >> 23) & 0xFF

  xm := (x & 0x7FFFFF) | 0x800000
  ym := (y & 0x7FFFFF) | 0x800000
  q1 := uint32((uint64(xm) * (uint64(1) << 25) / uint64(ym)))

  e1 := (xe - ye) + 126
  var q2 uint32
  if ((q1 & (1 << 25)) != 0) {
    e1++
    q2 = (q1 >> 1) & 0xFFFFFF
  } else {
    q2 = q1 & 0xFFFFFF
  }
  q3 := q2 + 1

  if (xe == 0) {
    return 0
  } else if (ye == 0) {
    return sign | (0xFF << 23)
  } else if ((e1 & 0x100) == 0) {
    return sign | ((e1 & 0xFF) << 23) | (q3 >> 1)
  } else if ((e1 & 0x80) == 0) {
    return sign | (0xFF << 23) | (q2 >> 1)
  } else {
    return 0
  }
}

func idiv( x, y uint32, signed_div bool) (uint32, uint32) {
  sign := (x < 0) && signed_div
  var x0 uint32
  if sign {
    x0 = -x
  }else{
    x0 = x
  }
 

  RQ := uint64(x0)
  for S := 0; S < 31; S++ {
    w0 := uint32(RQ >> 31)
    w1 := w0 - y
    if (w1 < 0) {
      RQ = (uint64(w0) << 32) | ((RQ & 0x7FFFFFFF) << 1)
    } else {
      RQ = (uint64(w1) << 32) | ((RQ & 0x7FFFFFFF) << 1) | 1
    }
  }

  quot :=  uint32(RQ)
  rem :=  uint32(RQ >> 32) 
  if (sign) {
    quot = -quot
    if (rem!=0) {
      quot -= 1
      rem = y - rem
    }
  }
  return quot,rem
}

func (risc *CORE) set_register(reg uint32, value uint32) {
  risc.R[reg] = value
  risc.Z = value == 0
  risc.N = int32(value) < 0
}




func (risc *CORE) Step(mb *board.BOARD,verbose bool) {
  
  var ir uint32
  switch{
  case risc.PC < board.MemWords :
    ir = mb.RAM[risc.PC];
  case (risc.PC >= board.ROMStart/4) && (risc.PC < board.ROMStart/4 + board.ROMWords) : 
    ir = mb.ROM[risc.PC - board.ROMStart/4]
  default: 
    fmt.Printf("Branched into the void (PC=0x%08X, I=%d), resetting...\n", risc.PC,risc.icount)
    mb.Reset(mb.Fbw,mb.Fbh,mb.Vchan,mb.PIchan,verbose)
    risc.Reset(int(risc.Cid),verbose)
    return
  }

  if verbose {fmt.Printf("\n%s %x, %x ","step",risc.PC,ir)}
  risc.PC=risc.PC+1
//  rtrace(&risc,ir)
  risc.icount++


  if ir & pbit == 0 {
    // Register instructions
    a  := (ir & 0x0F000000) >> 24
    b  := (ir & 0x00F00000) >> 20
    op := (ir & 0x000F0000) >> 16
    im :=  ir & 0x0000FFFF
    c  :=  ir & 0x0000000F

    var a_val, b_val, c_val uint32
    b_val = risc.R[b];
    if ((ir & qbit) == 0) {
      c_val = risc.R[c];
    } else if ((ir & vbit) == 0) {
      c_val = im;
    } else {
      c_val = 0xFFFF0000 | im;
    }
    
    switch op {
    case MOV:
        if ((ir & ubit) == 0) {
          a_val = c_val
        } else if ((ir & qbit) != 0) {
          a_val = c_val << 16;
        } else if ((ir & vbit) != 0) {
          a_val = 0xD0
	  if risc.N { a_val = a_val | 0x80000000 }
          if risc.Z { a_val = a_val | 0x40000000 }
          if risc.C { a_val = a_val | 0x20000000 }
          if risc.V { a_val = a_val | 0x10000000 }
        } else {
          a_val = risc.H;
        }
     
      
    case LSL: 
        a_val = b_val << (c_val & 31)
      
    case ASR: 
        a_val = uint32(int32((b_val)) >> (c_val & 31))
      
    case ROR: 
        a_val = (b_val >> (c_val & 31)) | (b_val << (-c_val & 31));
     
    case AND: 
        a_val = b_val & c_val
        
    case ANN: 
        a_val = b_val & ^c_val
     
    case IOR: 
        a_val = b_val | c_val
        
    case XOR: 
        a_val = b_val ^ c_val
        
    case ADD: 
        a_val = b_val + c_val
        if (((ir & ubit) != 0)&&risc.C) {
          a_val = a_val + 1
        }
        risc.C = a_val < b_val
        risc.V = (((a_val ^ c_val) & (a_val ^ b_val)) >> 31) != 0
        
    case SUB: 
        a_val = b_val - c_val
        if (((ir & ubit) != 0)&&risc.C) {
          a_val = a_val - 1
        }
        risc.C = a_val > b_val
        risc.V = (((b_val ^ c_val) & (a_val ^ b_val)) >> 31) != 0
       
      
    case MUL: 
        if ((ir & ubit) == 0) {
          tmpi := int64(int32(b_val)) * int64(int32(c_val))
          a_val = uint32(tmpi)
          risc.H = uint32(tmpi >> 32)
        } else {
          tmpu := uint64(b_val) * uint64(c_val)
          a_val = uint32(tmpu)
          risc.H = uint32(tmpu >> 32)
        }
      
    case DIV: 
        if (int32(c_val) > 0) {
          if ((ir & ubit) == 0) {
            a_val = uint32(int32(b_val) / int32(c_val))
            risc.H = uint32(int32(b_val) % int32(c_val))
            if (int32(risc.H) < 0) {
              a_val--
              risc.H += c_val
            }
          } else {
            a_val = b_val / c_val
            risc.H = b_val % c_val
          }
        } else {
          a_val,risc.H = idiv(b_val, c_val, (ir & ubit) != 0)
        }
        
    case FAD: 
	a_val = fp_add(b_val, c_val, (ir & ubit)!=0, (ir & vbit)!=0)
        
      
    case FSB: 
        a_val = fp_add(b_val, c_val ^ 0x80000000, (ir & ubit)!=0, (ir & vbit)!=0)
        
      
    case FML: 
        a_val = fp_mul(b_val, c_val)
       
      
    case FDV: 
        a_val = fp_div(b_val, c_val)
        
      
    default: 
      
    }
    risc.set_register( a, a_val)

  } else if ((ir & qbit) == 0) {
    // Memory instructions
    a := (ir & 0x0F000000) >> 24
    b := (ir & 0x00F00000) >> 20
    off := ir & 0x000FFFFF
    off = (off ^ 0x00080000) - 0x00080000 // sign-extend

    address := risc.R[b] + off
    if ((ir & ubit) == 0) {
      var a_val uint32

      if (ir & vbit) == 0 {
        a_val = mb.Load_word( address, 0)
      }else{ 
        a_val = uint32(mb.Load_byte( address, 0))
      }

      risc.set_register( a, a_val)

    }else{

      if (ir & vbit) == 0 {
        mb.Store_word(address, risc.R[a], 0)
      }else{
        mb.Store_byte(address, byte(risc.R[a]), 0)
      }

    }
  }else{
    // Branch instructions
    var t bool
    t = ((ir >> 27) & 1) != 0
    tf := (ir >> 24) & 7
    switch {
      case tf==0: t = t != risc.N
      case tf==1: t = t != risc.Z
      case tf==2: t = t != risc.C
      case tf==3: t = t != risc.V
      case tf==4: t = t != (risc.C || risc.Z)
      case tf==5: t = t != (risc.N != risc.V)
      case tf==6: t = t != ((risc.N != risc.V) || risc.Z)
      case tf==7: t = t != true
      default: //abort();  // unreachable
    }
    if (t) {
      if ((ir & vbit) != 0) {
        risc.set_register(15, risc.PC * 4);
      }
      if ((ir & ubit) == 0) {
        c := ir & 0x0000000F;
        risc.PC = risc.R[c] / 4;
      } else {
        off := ir & 0x00FFFFFF;
        off = (off ^ 0x00800000) - 0x00800000;  // sign-extend
        risc.PC = risc.PC + off;
      }
    }
  }
}
