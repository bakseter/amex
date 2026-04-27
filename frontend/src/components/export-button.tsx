import { useExportCsvMutation } from '@/api/transaction';

const ExportButton = () => {
    const [exportCsv, { isLoading, isSuccess }] = useExportCsvMutation();

    const handleClick = async () => {
        await exportCsv();
    };

    return (
        <div className="p-3 border-t border-(--color-border) shrink-0">
            <button
                onClick={handleClick}
                disabled={isLoading}
                className="w-full py-2 border border-(--color-border) rounded-sm text-[11px] tracking-[.08em] uppercase transition-colors hover:bg-(--color-border) hover:border-(--color-border2) disabled:opacity-40"
                style={
                    isSuccess
                        ? {
                              borderColor: 'var(--color-shared)',
                              color: 'var(--color-shared)',
                          }
                        : {}
                }
            >
                {isLoading && 'Exporting…'}
                {isSuccess && '✓'}
                {!isLoading && !isSuccess && 'Export CSV'}
            </button>
        </div>
    );
};

export default ExportButton;
