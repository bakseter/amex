import { type ChangeEvent,forwardRef } from 'react';

import {
    type Meta,
    type Transaction,
    usePatchTransactionMutation,
} from '@/api/transaction';
import { formatNumber, holderColor } from '@/utils/utils';

interface Props {
    transaction: Transaction;
    selected: boolean;
    onSelect: () => void;
    meta: Meta;
}

const TransactionRow = forwardRef(
    ({ transaction, selected, onSelect, meta }: Props, ref) => {
        const [patchTransaction] = usePatchTransactionMutation();

        const handleOnChangeCategory = async (
            event: ChangeEvent<HTMLSelectElement, HTMLSelectElement>
        ) => {
            await patchTransaction({
                id: transaction.id,
                fields: {
                    category: event.target.value,
                },
            });
        };

        const handleOnChangeCardholder = async (
            event: ChangeEvent<HTMLSelectElement, HTMLSelectElement>
        ) => {
            await patchTransaction({
                id: transaction.id,
                fields: {
                    cardholder: event.target.value,
                },
            });
        };

        const handleOnClickIsShared = async () => {
            await patchTransaction({
                id: transaction.id,
                fields: {
                    isShared: !transaction.isShared,
                },
            });
        };

        return (
            <tr
                ref={ref}
                className={`transaction-row border-b border-(--color-border) cursor-pointer ${
                    selected ? 'bg-[#1a1a24]' : 'hover:bg-(--color-surface)'
                }`}
                onClick={onSelect}
            >
                {/* Date */}
                <td className="px-3 py-2 text-[11px] text-(--color-muted) whitespace-nowrap tabular-nums">
                    {transaction.date}
                </td>

                {/* Description */}
                <td className="px-3 py-2 max-w-[260px]">
                    <span
                        className="block truncate text-[12px]"
                        title={transaction.description}
                    >
                        {transaction.description}
                    </span>
                </td>

                {/* Category — inline select */}
                <td
                    className="px-3 py-2"
                    onClick={(event) => {
                        event.stopPropagation();
                    }}
                >
                    <select
                        value={transaction.category}
                        onChange={handleOnChangeCategory}
                        className="bg-transparent border-none text-(--color-muted) font-mono text-[11px] outline-none cursor-pointer hover:text-(--color-text) max-w-[160px]"
                    >
                        {meta.categories.map((category) => (
                            <option
                                key={category}
                                value={category}
                                style={{ background: '#111114' }}
                            >
                                {category}
                            </option>
                        ))}
                    </select>
                </td>

                {/* Cardholder — inline select */}
                <td
                    className="px-3 py-2 whitespace-nowrap"
                    onClick={(event) => {
                        event.stopPropagation();
                    }}
                >
                    <select
                        value={transaction.cardholder}
                        onChange={handleOnChangeCardholder}
                        className="bg-transparent border-none font-mono text-[11px] outline-none cursor-pointer font-medium"
                        style={{ color: holderColor(transaction.cardholder) }}
                    >
                        {meta.cardholders.map((cardholder) => (
                            <option
                                key={cardholder}
                                value={cardholder}
                                style={{
                                    background: '#111114',
                                    color: holderColor(cardholder),
                                }}
                            >
                                {cardholder}
                            </option>
                        ))}
                    </select>
                </td>

                {/* Amount */}
                <td className="px-3 py-2 text-right tabular-nums text-[12px] whitespace-nowrap">
                    {formatNumber(transaction.amount)}
                </td>

                {/* Share toggle */}
                <td
                    className="px-3 py-2 text-center"
                    onClick={handleOnClickIsShared}
                >
                    <span
                        className="inline-block w-2.5 h-2.5 rounded-full transition-all duration-100 hover:scale-125 cursor-pointer"
                        style={
                            transaction.isShared
                                ? { background: 'var(--color-shared)' }
                                : {
                                      background: 'transparent',
                                      border: '1.5px solid var(--color-muted)',
                                  }
                        }
                        title={
                            transaction.isShared
                                ? 'Shared — click to make personal'
                                : 'Personal — click to share'
                        }
                    />
                </td>
            </tr>
        );
    }
);

export default TransactionRow;
