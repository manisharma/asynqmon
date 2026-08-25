import React, { useState } from "react";
import clsx from "clsx";
import { Link } from "react-router-dom";
import { makeStyles } from "@material-ui/core/styles";
import Table from "@material-ui/core/Table";
import TableBody from "@material-ui/core/TableBody";
import TableCell from "@material-ui/core/TableCell";
import TableContainer from "@material-ui/core/TableContainer";
import TableHead from "@material-ui/core/TableHead";
import TableRow from "@material-ui/core/TableRow";
import TableSortLabel from "@material-ui/core/TableSortLabel";
import TablePagination from "@material-ui/core/TablePagination";
import TextField from "@material-ui/core/TextField";
import Button from "@material-ui/core/Button";
import IconButton from "@material-ui/core/IconButton";
import Tooltip from "@material-ui/core/Tooltip";
import PauseCircleFilledIcon from "@material-ui/icons/PauseCircleFilled";
import PlayCircleFilledIcon from "@material-ui/icons/PlayCircleFilled";
import DeleteIcon from "@material-ui/icons/Delete";
import MoreHorizIcon from "@material-ui/icons/MoreHoriz";
import DeleteQueueConfirmationDialog from "./DeleteQueueConfirmationDialog";
import { Queue } from "../api";
import { queueDetailsPath } from "../paths";
import { SortDirection, SortableTableColumn } from "../types/table";
import prettyBytes from "pretty-bytes";
import { percentage } from "../utils";

const useStyles = makeStyles((theme) => ({
  table: {
    minWidth: 650,
  },
  searchControls: {
    display: "flex",
    alignItems: "center",
    gap: theme.spacing(1),
  },
  fixedCell: {
    position: "sticky",
    zIndex: 1,
    left: 0,
    background: theme.palette.background.paper,
  },
}));

interface QueueWithMetadata extends Queue {
  requestPending: boolean; // indicates pause/resume/delete request is pending for the queue.
}

interface Props {
  queues: QueueWithMetadata[];
  onPauseClick: (qname: string) => Promise<void>;
  onResumeClick: (qname: string) => Promise<void>;
  onDeleteClick: (qname: string) => Promise<void>;
  page: number;
  pageSize: number;
  total: number;
  search: string;
  taskID: string;
  onPageChange: (page: number) => void;
  onSearchChange: (search: string) => void;
  onTaskIDChange: (taskID: string) => void;
  onTaskIDSearch: () => void;
  sortBy: string;
  sortDir: SortDirection;
  onSortChange: (sortBy: string, sortDir: SortDirection) => void;
}

enum SortBy {
  Queue,
  State,
  Size,
  MemoryUsage,
  Latency,
  Processed,
  Failed,
  ErrorRate,

  None, // no sort support
}

const colConfigs: SortableTableColumn<SortBy>[] = [
  { label: "Queue", key: "queue", sortBy: SortBy.Queue, align: "left" },
  { label: "State", key: "state", sortBy: SortBy.State, align: "left" },
  {
    label: "Size",
    key: "size",
    sortBy: SortBy.Size,
    align: "right",
  },
  {
    label: "Memory usage",
    key: "memory_usage",
    sortBy: SortBy.MemoryUsage,
    align: "right",
  },
  {
    label: "Latency",
    key: "latency",
    sortBy: SortBy.Latency,
    align: "right",
  },
  {
    label: "Processed",
    key: "processed",
    sortBy: SortBy.Processed,
    align: "right",
  },
  { label: "Failed", key: "failed", sortBy: SortBy.Failed, align: "right" },
  {
    label: "Error rate",
    key: "error_rate",
    sortBy: SortBy.ErrorRate,
    align: "right",
  },
  { label: "Actions", key: "actions", sortBy: SortBy.None, align: "center" },
];

export default function QueuesOverviewTable(props: Props) {
  const classes = useStyles();
  const [queueToDelete, setQueueToDelete] = useState<QueueWithMetadata | null>(
    null
  );
  const handleDialogClose = () => {
    setQueueToDelete(null);
  };

  const sortKey = (sortBy: SortBy) =>
    colConfigs.find((column) => column.sortBy === sortBy)?.key || "queue";

  return (
    <React.Fragment>
      <TableContainer>
        <div className={classes.searchControls}>
          <TextField
            label="Search queues"
            value={props.search}
            onChange={(event) => props.onSearchChange(event.target.value)}
            variant="outlined"
            margin="dense"
          />
          <TextField
            label="Task ID"
            value={props.taskID}
            onChange={(event) => props.onTaskIDChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                props.onTaskIDSearch();
              }
            }}
            variant="outlined"
            margin="dense"
          />
          <Button
            color="primary"
            variant="contained"
            onClick={props.onTaskIDSearch}
          >
            Find task
          </Button>
        </div>
        <Table className={classes.table} aria-label="queues overview table">
          <TableHead>
            <TableRow>
              {colConfigs
                .filter((cfg) => {
                  // Filter out actions column in readonly mode.
                  return !window.READ_ONLY || cfg.key !== "actions";
                })
                .map((cfg, i) => (
                  <TableCell
                    key={cfg.key}
                    align={cfg.align}
                    className={clsx(i === 0 && classes.fixedCell)}
                  >
                    {cfg.sortBy !== SortBy.None ? (
                      <TableSortLabel
                        active={props.sortBy === cfg.key}
                        direction={props.sortDir}
                        onClick={() => {
                          const nextDir =
                            props.sortBy === cfg.key &&
                            props.sortDir === SortDirection.Asc
                              ? SortDirection.Desc
                              : SortDirection.Asc;
                          props.onSortChange(sortKey(cfg.sortBy), nextDir);
                        }}
                      >
                        {cfg.label}
                      </TableSortLabel>
                    ) : (
                      <div>{cfg.label}</div>
                    )}
                  </TableCell>
                ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {props.queues.map((q) => (
              <Row
                key={q.queue}
                queue={q}
                onPauseClick={() => props.onPauseClick(q.queue)}
                onResumeClick={() => props.onResumeClick(q.queue)}
                onDeleteClick={() => setQueueToDelete(q)}
              />
            ))}
          </TableBody>
        </Table>
        <TablePagination
          component="div"
          count={props.total}
          page={props.page - 1}
          onPageChange={(_, page) => props.onPageChange(page + 1)}
          rowsPerPage={props.pageSize}
          rowsPerPageOptions={[props.pageSize]}
        />
      </TableContainer>
      <DeleteQueueConfirmationDialog
        onClose={handleDialogClose}
        queue={queueToDelete}
      />
    </React.Fragment>
  );
}

const useRowStyles = makeStyles((theme) => ({
  row: {
    "&:last-child td": {
      borderBottomWidth: 0,
    },
    "&:last-child th": {
      borderBottomWidth: 0,
    },
  },
  linkText: {
    textDecoration: "none",
    color: theme.palette.text.primary,
    "&:hover": {
      textDecoration: "underline",
    },
  },
  textGreen: {
    color: theme.palette.success.dark,
  },
  textRed: {
    color: theme.palette.error.dark,
  },
  boldCell: {
    fontWeight: 600,
  },
  fixedCell: {
    position: "sticky",
    zIndex: 1,
    left: 0,
    background: theme.palette.background.paper,
  },
  actionIconsContainer: {
    display: "flex",
    justifyContent: "center",
    minWidth: "100px",
  },
}));

interface RowProps {
  queue: QueueWithMetadata;
  onPauseClick: () => void;
  onResumeClick: () => void;
  onDeleteClick: () => void;
}

function Row(props: RowProps) {
  const classes = useRowStyles();
  const { queue: q } = props;
  const [showIcons, setShowIcons] = useState<boolean>(false);
  return (
    <TableRow key={q.queue} className={classes.row}>
      <TableCell
        component="th"
        scope="row"
        className={clsx(classes.boldCell, classes.fixedCell)}
      >
        <Link to={queueDetailsPath(q.queue)} className={classes.linkText}>
          {q.queue}
        </Link>
      </TableCell>
      <TableCell>
        {q.paused ? (
          <span className={classes.textRed}>paused</span>
        ) : (
          <span className={classes.textGreen}>run</span>
        )}
      </TableCell>
      <TableCell align="right">{q.size}</TableCell>
      <TableCell align="right">{prettyBytes(q.memory_usage_bytes)}</TableCell>
      <TableCell align="right">{q.display_latency}</TableCell>
      <TableCell align="right">{q.processed}</TableCell>
      <TableCell align="right">{q.failed}</TableCell>
      <TableCell align="right">{percentage(q.failed, q.processed)}</TableCell>
      {!window.READ_ONLY && (
        <TableCell
          align="center"
          onMouseEnter={() => setShowIcons(true)}
          onMouseLeave={() => setShowIcons(false)}
        >
          <div className={classes.actionIconsContainer}>
            {showIcons ? (
              <React.Fragment>
                {q.paused ? (
                  <Tooltip title="Resume">
                    <IconButton
                      color="secondary"
                      onClick={props.onResumeClick}
                      disabled={q.requestPending}
                      size="small"
                    >
                      <PlayCircleFilledIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                ) : (
                  <Tooltip title="Pause">
                    <IconButton
                      color="primary"
                      onClick={props.onPauseClick}
                      disabled={q.requestPending}
                      size="small"
                    >
                      <PauseCircleFilledIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                )}
                <Tooltip title="Delete">
                  <IconButton onClick={props.onDeleteClick} size="small">
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </React.Fragment>
            ) : (
              <IconButton size="small">
                <MoreHorizIcon fontSize="small" />
              </IconButton>
            )}
          </div>
        </TableCell>
      )}
    </TableRow>
  );
}
