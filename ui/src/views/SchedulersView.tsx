import React, { useCallback, useState } from "react";
import { connect, ConnectedProps } from "react-redux";
import Container from "@material-ui/core/Container";
import { makeStyles } from "@material-ui/core/styles";
import Grid from "@material-ui/core/Grid";
import Paper from "@material-ui/core/Paper";
import SchedulerEntriesTable from "../components/SchedulerEntriesTable";
import Typography from "@material-ui/core/Typography";
import Alert from "@material-ui/lab/Alert";
import AlertTitle from "@material-ui/lab/AlertTitle";
import TablePagination from "@material-ui/core/TablePagination";
import TextField from "@material-ui/core/TextField";
import { AppState } from "../store";
import { listSchedulerEntriesAsync } from "../actions/schedulerEntriesActions";
import { usePolling } from "../hooks";

const useStyles = makeStyles((theme) => ({
  container: {
    paddingTop: theme.spacing(4),
    paddingBottom: theme.spacing(4),
  },
  paper: {
    padding: theme.spacing(2),
    display: "flex",
    overflow: "auto",
    flexDirection: "column",
  },
  heading: {
    paddingLeft: theme.spacing(2),
    marginBottom: theme.spacing(1),
  },
}));

function mapStateToProps(state: AppState) {
  return {
    loading: state.schedulerEntries.loading,
    error: state.schedulerEntries.error,
    entries: state.schedulerEntries.data,
    pageSize: state.schedulerEntries.size,
    total: state.schedulerEntries.total,
    pollInterval: state.settings.pollInterval,
  };
}

const connector = connect(mapStateToProps, { listSchedulerEntriesAsync });

type Props = ConnectedProps<typeof connector>;

function SchedulersView(props: Props) {
  const { pollInterval, listSchedulerEntriesAsync, pageSize, total } = props;
  const classes = useStyles();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");

  const listSchedulerEntries = useCallback(
    () => listSchedulerEntriesAsync({ page, size: pageSize, search }),
    [listSchedulerEntriesAsync, page, pageSize, search]
  );
  usePolling(listSchedulerEntries, pollInterval);

  return (
    <Container maxWidth="lg" className={classes.container}>
      <Grid container spacing={3}>
        {props.error === "" ? (
          <Grid item xs={12}>
            <Paper className={classes.paper} variant="outlined">
              <Typography variant="h6" className={classes.heading}>
                Scheduler Entries
              </Typography>
              <TextField
                label="Search entry ID, spec, or task type"
                value={search}
                onChange={(event) => {
                  setSearch(event.target.value);
                  setPage(1);
                }}
                variant="outlined"
                margin="dense"
              />
              <SchedulerEntriesTable entries={props.entries} />
              <TablePagination
                component="div"
                count={total}
                page={page - 1}
                onPageChange={(_, nextPage) => setPage(nextPage + 1)}
                rowsPerPage={pageSize}
                rowsPerPageOptions={[pageSize]}
              />
            </Paper>
          </Grid>
        ) : (
          <Grid item xs={12}>
            <Alert severity="error">
              <AlertTitle>Error</AlertTitle>
              Could not retrieve scheduler entries live data —{" "}
              <strong>See the logs for details</strong>
            </Alert>
          </Grid>
        )}
      </Grid>
    </Container>
  );
}

export default connector(SchedulersView);
