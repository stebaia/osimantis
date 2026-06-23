import 'package:dio/dio.dart';

import '../../../../core/error/failure.dart';
import '../../../../core/error/result.dart';
import '../../../../core/network/api_exception.dart';
import '../../domain/entities/person_card.dart';
import '../../domain/repositories/person_repository.dart';
import '../datasources/person_remote_datasource.dart';

/// Implementazione del PersonRepository: delega al datasource e converte le
/// eccezioni tecniche in Result/Failure di dominio.
class PersonRepositoryImpl implements PersonRepository {
  PersonRepositoryImpl(this._remote);
  final PersonRemoteDataSource _remote;

  @override
  Future<Result<PersonCard>> getPerson(int id) async {
    try {
      return Success(await _remote.getPerson(id));
    } on DioException catch (e) {
      return Error(mapDioError(e));
    } catch (_) {
      return const Error(ServerFailure());
    }
  }
}
