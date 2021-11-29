package vm

import (
	"os"
	"path/filepath"
	"reflect"

	"github.com/gnolang/gno"
	"github.com/gnolang/gno/pkgs/crypto"
	osm "github.com/gnolang/gno/pkgs/os"
	"github.com/gnolang/gno/pkgs/std"
	"github.com/gnolang/gno/stdlibs"
)

func (vmk *VMKeeper) initBuiltinPackages(store gno.Store) {
	// NOTE: native functions/methods added here must be quick operations,
	// or account for gas before operation.
	// TODO: define criteria for inclusion, and solve gas calculations.
	getPackage := func(pkgPath string) (pv *gno.PackageValue) {
		// otherwise, built-in package value.
		// first, load from filepath.
		stdlibPath := filepath.Join("../../../stdlibs", pkgPath)
		if !osm.DirExists(stdlibPath) {
			// does not exist.
			return nil
		}
		memPkg := gno.ReadMemPackage(stdlibPath, pkgPath)
		m2 := gno.NewMachineWithOptions(gno.MachineOptions{
			Package: nil,
			Output:  os.Stdout,
			Store:   store,
		})
		m2.RunMemPackage(memPkg, true)
		pv = m2.Package
		// inject default native injections.
		stdlibs.InjectNatives(store, pv)
		// inject VMKeeper specific natives.
		pkg := store.GetBlockNode(gno.PackageNodeLocation(pv.PkgPath)).(*gno.PackageNode)
		switch pkgPath {
		case "std":
			// Native functions.
			pkg.DefineNative("Send",
				gno.Flds( // params
					"toAddr", "Address",
					"coins", "Coins",
				),
				gno.Flds( // results
					"err", "error",
				),
				func(m *gno.Machine) {
					if m.ReadOnly {
						panic("cannot send -- readonly")
					}
					arg0, arg1 := m.LastBlock().GetParams2()
					toAddrBz := arg0.TV.V.(*gno.ArrayValue).GetReadonlyBytes()
					if len(toAddrBz) != 20 {
						panic("unexpected address length")
					}
					toAddr := crypto.Address{}
					copy(toAddr[:], toAddrBz)
					sendGno := arg1.TV.V.(*gno.SliceValue)
					send := std.Coins(nil)
					sendSize := sendGno.GetLength()
					for i := 0; i < sendSize; i++ {
						coinGno := sendGno.GetPointerAtIndexInt2(store, i, nil).TV.V.(*gno.StructValue)
						denom := coinGno.Fields[0].GetString()
						amount := coinGno.Fields[1].GetInt64()
						send = append(send, std.Coin{Denom: denom, Amount: amount})
					}
					if !send.IsValid() {
						panic("invalid coins")
					}

					ctx := m.Context.(ExecContext)
					err := vmk.bank.SendCoins(
						ctx.sdkCtx,
						ctx.PkgAddr,
						toAddr,
						send,
					)
					if err != nil {
						res0 := gno.Go2GnoValue(
							reflect.ValueOf(err),
						)
						m.PushValue(res0)
					} else {
						m.PushValue(gno.TypedValue{})
					}
				},
			)
			pkg.DefineNative("GetChainID",
				gno.Flds( // params
				),
				gno.Flds( // results
					"", "string",
				),
				func(m *gno.Machine) {
					ctx := m.Context.(ExecContext)
					res0 := gno.Go2GnoValue(
						reflect.ValueOf(ctx.ChainID),
					)
					m.PushValue(res0)
				},
			)
			pkg.DefineNative("GetHeight",
				gno.Flds( // params
				),
				gno.Flds( // results
					"", "int64",
				),
				func(m *gno.Machine) {
					ctx := m.Context.(ExecContext)
					res0 := gno.Go2GnoValue(
						reflect.ValueOf(ctx.Height),
					)
					m.PushValue(res0)
				},
			)
			pkg.DefineNative("GetSend",
				gno.Flds( // params
				),
				gno.Flds( // results
					"", "Coins",
				),
				func(m *gno.Machine) {
					ctx := m.Context.(ExecContext)
					res0 := gno.Go2GnoValue(
						reflect.ValueOf(ctx.Msg.Send),
					)
					coinT := store.GetType(gno.DeclaredTypeID("std", "Coin"))
					coinsT := store.GetType(gno.DeclaredTypeID("std", "Coins"))
					res0.T = coinsT
					av := res0.V.(*gno.SliceValue).Base.(*gno.ArrayValue)
					for i, _ := range av.List {
						av.List[i].T = coinT
					}
					m.PushValue(res0)
				},
			)
			pkg.DefineNative("GetCaller",
				gno.Flds( // params
				),
				gno.Flds( // results
					"", "Address",
				),
				func(m *gno.Machine) {
					ctx := m.Context.(ExecContext)
					res0 := gno.Go2GnoValue(
						reflect.ValueOf(ctx.Msg.Caller),
					)
					addrT := store.GetType(gno.DeclaredTypeID("std", "Address"))
					res0.T = addrT
					m.PushValue(res0)
				},
			)
			pkg.DefineNative("GetPkgAddr",
				gno.Flds( // params
				),
				gno.Flds( // results
					"", "Address",
				),
				func(m *gno.Machine) {
					ctx := m.Context.(ExecContext)
					res0 := gno.Go2GnoValue(
						reflect.ValueOf(ctx.PkgAddr),
					)
					addrT := store.GetType(gno.DeclaredTypeID("std", "Address"))
					res0.T = addrT
					m.PushValue(res0)
				},
			)
			pkg.PrepareNewValues(pv)
			return pv
		default:
			return nil // no vm injections.
		}
	}
	store.SetPackageGetter(getPackage)
}
