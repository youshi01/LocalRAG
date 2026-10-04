import ConfirmDialog from '../common/ConfirmDialog'

interface ClearAllConversationsDialogProps {
  open: boolean
  conversationCount: number
  isClearing: boolean
  canConfirm: boolean
  onConfirm: () => void
  onCancel: () => void
}

const ClearAllConversationsDialog = ({
  open, conversationCount, isClearing, canConfirm, onConfirm, onCancel,
}: ClearAllConversationsDialogProps) => (
  <ConfirmDialog
    open={open}
    title="清空本项目全部会话"
    message={'当前共有 ' + conversationCount + ' 个会话（包括空白本地会话，不受搜索筛选影响）。确认清空本项目全部会话及其中的全部消息吗？这包括所有知识库范围内的会话。此操作不可恢复、不可撤销。知识库、文件、索引和设置均不受影响。'}
    confirmText={isClearing ? '正在清空…' : '确认清空本项目全部会话'}
    confirmDisabled={isClearing || !canConfirm}
    cancelDisabled={isClearing}
    onConfirm={onConfirm}
    onCancel={onCancel}
  />
)

export default ClearAllConversationsDialog
