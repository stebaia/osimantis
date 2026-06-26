import 'package:dio/dio.dart';

import '../../../../core/error/failure.dart';
import '../../../../core/error/result.dart';
import '../../../../core/network/api_exception.dart';
import '../../domain/entities/graph_data.dart';
import '../../domain/repositories/graph_repository.dart';
import '../datasources/graph_remote_datasource.dart';

/// Implementazione del GraphRepository: delega al datasource e converte le
/// eccezioni tecniche in Result/Failure di dominio.
class GraphRepositoryImpl implements GraphRepository {
  GraphRepositoryImpl(this._remote);
  final GraphRemoteDataSource _remote;

  @override
  Future<Result<GraphData>> getGraph() async {
    try {
      return Success(await _remote.getGraph());
    } on DioException catch (e) {
      return Error(mapDioError(e));
    } catch (_) {
      return const Error(ServerFailure());
    }
  }
}
