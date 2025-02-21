/*******************************************************
 *                       WARNING                       *
 *  Solidity to Yul compilation is still EXPERIMENTAL  *
 *       It can result in LOSS OF FUNDS or worse       *
 *                !USE AT YOUR OWN RISK!               *
 *******************************************************/


object "Simple_23" {
    code {
        mstore(64, 128)
        if callvalue() { revert(0, 0) }

        constructor_Simple_23()

        codecopy(0, dataoffset("Simple_23_deployed"), datasize("Simple_23_deployed"))

        return(0, datasize("Simple_23_deployed"))

        function cleanup_t_uint256(value) -> cleaned {
            cleaned := value
        }

        function constructor_Simple_23() {

            let expr_3 := 0xa000000000000000000000000000000000000000000000000000000000001000
            let _1 := convert_t_address_payable_to_t_address(expr_3)
            update_storage_value_offset_0t_address_to_t_address(0x00, _1)

        }

        function convert_t_address_payable_to_t_address(value) -> converted {
            converted := convert_t_uint256_to_t_address(value)
        }

        function convert_t_address_to_t_address(value) -> converted {
            converted := convert_t_uint256_to_t_address(value)
        }

        function convert_t_uint256_to_t_address(value) -> converted {
            converted := convert_t_uint256_to_t_uint256(value)
        }

        function convert_t_uint256_to_t_uint256(value) -> converted {
            converted := cleanup_t_uint256(value)
        }

        function prepare_store_t_address(value) -> ret {
            ret := value
        }

        function shift_left_0(value) -> newValue {
            newValue :=

            shl(0, value)

        }

        function update_byte_slice_32_shift_0(value, toInsert) -> result {
            let mask := 0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
            toInsert := shift_left_0(toInsert)
            value := and(value, not(mask))
            result := or(value, and(toInsert, mask))
        }

        function update_storage_value_offset_0t_address_to_t_address(slot, value_0) {
            let convertedValue_0 := convert_t_address_to_t_address(value_0)
            sstore(slot, update_byte_slice_32_shift_0(sload(slot), prepare_store_t_address(convertedValue_0)))
        }

    }
    object "Simple_23_deployed" {
        code {
            mstore(64, 128)

            if iszero(lt(calldatasize(), 4))
            {
                let selector := shift_right_224_unsigned(calldataload(0))
                switch selector

                case 0x38cc4831
                {
                    // getAddress()
                    if callvalue() { revert(0, 0) }
                    abi_decode_tuple_(4, calldatasize())
                    let ret_0 :=  fun_getAddress_12()
                    let memPos := allocateMemory(0)
                    let memEnd := abi_encode_tuple_t_address__to_t_address__fromStack(memPos , ret_0)
                    return(memPos, sub(memEnd, memPos))
                }

                case 0x449caa2b
                {
                    // getInputAddress(address)
                    if callvalue() { revert(0, 0) }
                    let param_0 :=  abi_decode_tuple_t_address(4, calldatasize())
                    let ret_0 :=  fun_getInputAddress_22(param_0)
                    let memPos := allocateMemory(0)
                    let memEnd := abi_encode_tuple_t_address__to_t_address__fromStack(memPos , ret_0)
                    return(memPos, sub(memEnd, memPos))
                }

                default {}
            }
            if iszero(calldatasize()) {  }
            revert(0, 0)

            function abi_decode_t_address(offset, end) -> value {
                value := calldataload(offset)
                validator_revert_t_address(value)
            }

            function abi_decode_tuple_(headStart, dataEnd)   {
                if slt(sub(dataEnd, headStart), 0) { revert(0, 0) }

            }

            function abi_decode_tuple_t_address(headStart, dataEnd) -> value0 {
                if slt(sub(dataEnd, headStart), 32) { revert(0, 0) }

                {

                    let offset := 0

                    value0 := abi_decode_t_address(add(headStart, offset), dataEnd)
                }

            }

            function abi_encode_t_address_to_t_address_fromStack(value, pos) {
                mstore(pos, cleanup_t_address(value))
            }

            function abi_encode_tuple_t_address__to_t_address__fromStack(headStart , value0) -> tail {
                tail := add(headStart, 32)

                abi_encode_t_address_to_t_address_fromStack(value0,  add(headStart, 0))

            }

            function allocateMemory(size) -> memPtr {
                memPtr := mload(64)
                let newFreePtr := add(memPtr, size)
                // protect against overflow
                if or(gt(newFreePtr, 0xffffffffffffffff), lt(newFreePtr, memPtr)) { panic_error() }
                mstore(64, newFreePtr)
            }

            function cleanup_from_storage_t_address(value) -> cleaned {
                cleaned := value
            }

            function cleanup_t_address(value) -> cleaned {
                cleaned := cleanup_t_uint256(value)
            }

            function cleanup_t_uint256(value) -> cleaned {
                cleaned := value
            }

            function extract_from_storage_value_offset_0t_address(slot_value) -> value {
                value := cleanup_from_storage_t_address(shift_right_0_unsigned(slot_value))
            }

            function fun_getAddress_12() -> vloc__7 {
                let zero_value_for_type_t_address_1 := zero_value_for_split_t_address()
                vloc__7 := zero_value_for_type_t_address_1

                let _2 := read_from_storage_split_offset_0_t_address(0x00)
                let expr_9 := _2
                vloc__7 := expr_9
                leave

            }

            function fun_getInputAddress_22(vloc_input_14) -> vloc__17 {
                let zero_value_for_type_t_address_3 := zero_value_for_split_t_address()
                vloc__17 := zero_value_for_type_t_address_3

                let _4 := vloc_input_14
                let expr_19 := _4
                vloc__17 := expr_19
                leave

            }

            function panic_error() {
                invalid()
            }

            function read_from_storage_split_offset_0_t_address(slot) -> value {
                value := extract_from_storage_value_offset_0t_address(sload(slot))

            }

            function shift_right_0_unsigned(value) -> newValue {
                newValue :=

                shr(0, value)

            }

            function shift_right_224_unsigned(value) -> newValue {
                newValue :=

                shr(224, value)

            }

            function validator_revert_t_address(value) {
                if iszero(eq(value, cleanup_t_address(value))) { revert(0, 0) }
            }

            function zero_value_for_split_t_address() -> ret {
                ret := 0
            }

        }

    }

}

