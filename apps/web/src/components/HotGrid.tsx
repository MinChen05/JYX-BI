import Handsontable from 'handsontable';
import 'handsontable/styles/handsontable.min.css';
import 'handsontable/styles/theme/main.min.css';
import React, {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
} from 'react';

import type { ColumnSpec, RowOps } from '@kingdee-rpt/rpt-types';

export interface GridHandle {
  /** 取当前网格全部数据（行序 = rowKeys 顺序） */
  getData: () => any[][];
  /** 追加一行（返回新行下标） */
  insertRowEnd: () => number;
  /** 删除选中行 */
  deleteSelected: () => void;
  /** 选中行下标列表 */
  selectedRows: () => number[];
}

interface Props {
  columns: ColumnSpec[];
  rowKeys: string[];
  initialData: any[][];
  rowOps: RowOps;
  disabled?: boolean;
  onData?: (data: any[][], rowKeys: string[]) => void;
}

const HotGrid = forwardRef<GridHandle, Props>(
  ({ columns, rowKeys, initialData, rowOps, disabled, onData }, ref) => {
    const holderRef = useRef<HTMLDivElement>(null);
    const hotRef = useRef<Handsontable | null>(null);
    const rowKeysRef = useRef(rowKeys);
    rowKeysRef.current = rowKeys;
    const onDataRef = useRef(onData);
    onDataRef.current = onData;

    useImperativeHandle(ref, () => ({
      getData: () => (hotRef.current?.getData() as any[][]) ?? [],
      insertRowEnd: () => {
        const last = (hotRef.current?.countRows() ?? 1) - 1;
        hotRef.current?.alter('insert_row_below', last);
        return last + 1;
      },
      deleteSelected: () => {
        const sel = hotRef.current?.getSelected() ?? [];
        if (sel.length > 0) {
          const [start, , end] = sel[0];
          hotRef.current?.alter('remove_row', start, end - start + 1);
        }
      },
      selectedRows: () => {
        const sel = hotRef.current?.getSelected() ?? [];
        if (sel.length === 0) return [];
        const [start, , end] = sel[0];
        const out: number[] = [];
        for (let r = start; r <= end; r++) out.push(r);
        return out;
      },
    }));

    // 初始化和数据变化
    useEffect(() => {
      if (!holderRef.current) return;
      const hot = new Handsontable(holderRef.current, {
        data: initialData,
        colHeaders: columns.map((c) => c.label),
        columns: columns.map((c) => ({
          type:
            c.type === 'money' || c.type === 'int' ? 'numeric' : 'text',
          readOnly: c.readonly || disabled,
        })),
        rowHeaders: false,
        colWidths: columns.map((c) => c.width ?? 120),
        manualRowResize: true,
        stretchH: 'all',
        height: '100%',
        licenseKey: 'non-commercial-and-evaluation',
        afterChange: (changes) => {
          if (!changes || disabled) return;
          const data = hot.getData() as any[][];
          onDataRef.current?.(data, rowKeysRef.current);
        },
      });
      hotRef.current = hot;
      return () => {
        hot.destroy();
        hotRef.current = null;
      };
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    // 列结构变化（动态列：业务日期切换）
    useEffect(() => {
      if (!hotRef.current) return;
      hotRef.current.updateSettings({
        colHeaders: columns.map((c) => c.label),
        columns: columns.map((c) => ({
          type:
            c.type === 'money' || c.type === 'int' ? 'numeric' : 'text',
          readOnly: c.readonly || disabled,
        })),
        colWidths: columns.map((c) => c.width ?? 120),
      });
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [columns, disabled]);

    // 行数据整体刷新
    useEffect(() => {
      hotRef.current?.loadData(initialData);
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [initialData]);

    return <div className="rpt-hotgrid" ref={holderRef} />;
  },
);

HotGrid.displayName = 'HotGrid';
export default HotGrid;
