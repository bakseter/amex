// components/edit-select.tsx
const EditSelect = ({
    value,
    options,
    onChange,
    color,
    tone = 'default',
}: {
    value: string;
    options: string[];
    onChange: (value: string) => void;
    color?: string;
    tone?: 'default' | 'attention';
}) => (
    <select
        value={value}
        onChange={(event) => {
            onChange(event.target.value);
        }}
        onClick={(event) => {
            event.stopPropagation();
        }}
        style={color ? { color } : undefined}
        className={`max-w-[160px] cursor-pointer truncate rounded-md border bg-transparent px-1.5 py-1 focus:outline-none ${
            tone === 'attention'
                ? 'border-transparent text-danger hover:bg-surface'
                : 'border-transparent hover:bg-surface'
        }`}
    >
        {options.map((option) => (
            <option key={option} value={option}>
                {option}
            </option>
        ))}
    </select>
);

export default EditSelect;
