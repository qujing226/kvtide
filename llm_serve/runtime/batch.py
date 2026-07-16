from dataclasses import dataclass

from mini_llm_serve.v1 import executor_pb2


@dataclass
class BatchMetadata:
    items: list[executor_pb2.ExecuteItem]

    # flatten all token_ids
    input_ids: list[int]
    # the sequence pos for each token in input_ids
    positions: list[int]

    # query_start_locs is the boundry of each flattened input_ids
    # list[0] ~ list[1] point the first item.
    # list[1] ~ list[2] point the second item.
    query_start_locs: list[int]

    # total context length after current execution done.
    context_lens: list[int]
    # physical slot of current token
    slot_mapping: list[int]
    block_tables: list[list[int]]

    # pos of the logits need to sample in the flattened token
    sample_indices: list[int]
    # which item in the original items does sample_indices correspond to
    sample_item_indices: list[int]


class BatchBuilder:
    def __init__(self, block_size: int):
        if block_size <= 0:
            raise ValueError("block_size must be positive")
        self.block_size = block_size

    def build(
        self,
        items: list[executor_pb2.ExecuteItem],
    ) -> BatchMetadata:
        input_ids: list[int] = []
        positions: list[int] = []
        query_start_locs: list[int] = [0]
        context_lens: list[int] = []
        slot_mapping: list[int] = []
        block_tables: list[list[int]] = []
        sample_indices: list[int] = []
        sample_item_indices: list[int] = []

        for item_index, item in enumerate(items):
            tokens = list(item.token_ids)
            block_table = list(item.kv_blocks.block_table)

            if not tokens:
                raise ValueError("execute item must contain tokens")
            if len(tokens) != item.num_new_tokens:
                raise ValueError("token count does not match num_new_tokens")
            if item.kv_blocks.block_size != self.block_size:
                raise ValueError("block size mismatch")

            # content length
            start = item.computed_tokens
            end = start + len(tokens)
            capacity = len(block_table) * self.block_size

            if end > capacity:
                raise ValueError("block table capacity is insufficient")

            # flatten, e.g.
            # computed_tokens = 45
            # input_ids: [..., 90, 281]
            # positiong: [..., 45, 46]
            # query_start_locs: [0, ..., a, a+b, a+b+2 ]
            # context_len: [..., 47]
            input_ids.extend(tokens)
            positions.extend(range(start, end))
            query_start_locs.append(query_start_locs[-1] + len(tokens))
            context_lens.append(end)

            # transfer token pos into slot pos for each token, e.g.
            # block_table: [1, 3], position: 5, block_size = 4
            # logical_block = 5 // 4 = 1, block_offset = 5 % 4 = 1
            # physical_slot = block_table[1] * block_size + 1 = 13
            for position in range(start, end):
                logical_block = position // self.block_size
                block_offset = position % self.block_size
                physical_block = block_table[logical_block]
                slot_mapping.append(physical_block * self.block_size + block_offset)

            # How to compute history length?
            # use end pos - input length
            # query_len = query_start_locs[i + 1] - query_start_locs[i]
            # history_len = context_lens[i] - query_len

            # record which item the output logit belongs to.
            if item.sample:
                sample_indices.append(len(input_ids) - 1)
                sample_item_indices.append(item_index)

            block_tables.append(block_table)

        # padding
        max_blocks = max((len(table) for table in block_tables), default=0)
        block_tables = [
            table + [-1] * (max_blocks - len(table)) for table in block_tables
        ]

        return BatchMetadata(
            items=items,
            input_ids=input_ids,
            positions=positions,
            query_start_locs=query_start_locs,
            context_lens=context_lens,
            slot_mapping=slot_mapping,
            block_tables=block_tables,
            sample_indices=sample_indices,
            sample_item_indices=sample_item_indices,
        )
